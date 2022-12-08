<script>
import axios from "axios";
import { pushNotFound } from "../assets/utils";
import { useMastodonStore } from "../stores/mastodon";
import { map, some, omit } from "lodash-es";
import Participant from "../components/Participant.vue";
import {
  mdiMicrophone,
  mdiMicrophoneOff,
  mdiPhoneRemove,
  mdiMicrophoneQuestion,
} from "@mdi/js";
import { Room, RoomEvent, Track } from "livekit-client";
import { login } from "masto";

export default {
  setup() {
    return {
      donStore: useMastodonStore(),
    };
  },
  components: {
    Participant,
  },
  data() {
    return {
      mdiMicrophone,
      mdiMicrophoneOff,
      mdiPhoneRemove,
      mdiMicrophoneQuestion,
      roomID: this.$route.params.id,
      loading: false,
      mainHeight: 600,
      roomClient: null,
      roomInfo: {
        title: "",
        description: "",
        host: null,
        cohosts: [],
        createdAt: null,
      },
      participants: {},
      cachedMastoData: {},
      activeSpeakerIDs: new Set(),
    };
  },
  created() {
    // watch the params of the route to fetch the data again
    this.$watch(
      () => this.$route.params,
      () => {
        this.joinRoom();
      },
      // fetch the data when the view is created and the data is
      // already being observed
      { immediate: true }
    );
  },
  mounted() {
    this.onResize();
  },
  methods: {
    async joinRoom() {
      this.loading = true;
      try {
        const resp = await axios.get(`/api/room/${this.roomID}`);
        const room = new Room({
          adaptiveStream: true,
          dynacast: true,
          publishDefaults: {
            stopMicTrackOnMute: true,
            simulcast: false,
          },
        });
        const self = this;
        room
          .on(RoomEvent.TrackSubscribed, (track, publication, participant) => {
            if (track.kind === Track.Kind.Audio) {
              const element = track.attach();
              self.$refs.audioDOM.appendChild(element);
            }
          })
          .on(
            RoomEvent.TrackUnsubscribed,
            (track, publication, participant) => {
              track.detach();
            }
          )
          .on(RoomEvent.LocalTrackUnpublished, (publication, participant) => {
            publication.track?.detach();
          })
          .on(RoomEvent.ActiveSpeakersChanged, (speakers) => {
            self.activeSpeakerIDs = new Set(map(speakers, (p) => p.identity));
          })
          .on(RoomEvent.ParticipantConnected, (participant) => {
            const metadata = self.addParticipant(participant);
            if (metadata !== null) {
              self.fetchMastoData(participant.identity, metadata);
            }
          })
          .on(RoomEvent.ParticipantDisconnected, (participant) => {
            self.participants = omit(self.participants, participant.identity);
          })
          .on(RoomEvent.AudioPlaybackStatusChanged, () => {
            if (!room.canPlaybackAudio) {
              // FIXME: popup a dialog to ask user to allow audio playback
              console.log("needs audio playback permission");
            }
          })
          .on(RoomEvent.Disconnected, (reason) => {
            console.log("disconnected: ", reason);
          });
        await room.connect(resp.data.url, resp.data.token);
        this.roomClient = room;
        this.roomInfo = JSON.parse(room.metadata);
        this.addParticipant(room.localParticipant);
        for (const part of room.participants.values()) {
          this.addParticipant(part);
        }
        this.activeSpeakerIDs = new Set(
          map(room.activeSpeakers, (p) => p.identity)
        );
        // cache mastodon data of current participants
        for (const [key, value] of Object.entries(this.participants)) {
          if (value !== null) {
            this.fetchMastoData(key, value);
          }
        }
      } catch (error) {
        if (error.response?.status === 404) {
          pushNotFound(this.$route);
        } else {
          console.log(error);
        }
      } finally {
        this.loading = false;
      }
    },
    onResize() {
      const mainArea = document.getElementById("mainArea");
      const height = mainArea.clientHeight;
      this.mainHeight = height > 700 ? 700 : window.innerHeight - 70;
    },
    isCohost(value) {
      return (
        value &&
        some(this.roomInfo.cohosts, {
          remote_id: value.remote_id,
          remote_url: value.remote_url,
        })
      );
    },
    addParticipant(participant) {
      const metadata = participant.metadata
        ? JSON.parse(participant.metadata)
        : null;
      this.participants[participant.identity] = metadata;
      return metadata;
    },
    async fetchMastoData(identity, { remote_id, remote_url }) {
      if (this.cachedMastoData[identity] !== undefined) return;
      try {
        const url = new URL(remote_url);
        const mastoClient = await login({
          url: url.origin,
          disableVersionCheck: true,
        });
        const info = await mastoClient.accounts.fetch(remote_id);
        this.cachedMastoData[identity] = info;
      } catch (error) {
        // FIXME: display error snackbar
        console.log(error);
      }
    },
  },
};
</script>

<template>
  <div class="d-none" ref="audioDOM"></div>
  <main class="fill-height" v-resize="onResize">
    <v-card :height="mainHeight" :loading="loading" class="d-flex flex-column">
      <v-card-title>{{ roomInfo.title }}</v-card-title>
      <div
        class="overflow-auto flex-shrink-0 pb-2"
        v-if="roomInfo.description"
        style="height: 100px"
      >
        <v-container class="py-0">
          <pre class="text-body-1">{{ roomInfo.description }}</pre>
        </v-container>
      </div>
      <v-divider></v-divider>
      <v-card-text class="flex-grow-1 overflow-auto">
        <v-row justify="start">
          <template v-for="(value, key) of participants" :key="key">
            <Participant
              v-if="key === roomInfo.host?.audon_id"
              :talking="activeSpeakerIDs.has(key)"
              type="host"
              :data="cachedMastoData[key]"
            ></Participant>
            <Participant
              v-if="isCohost(value)"
              :talking="activeSpeakerIDs.has(key)"
              type="cohost"
              :data="cachedMastoData[key]"
            ></Participant>
          </template>
        </v-row>
        <v-row>
          <template v-for="(value, key) of participants" :key="key">
            <Participant
              v-if="key !== roomInfo.host?.audon_id && !isCohost(value)"
              :talking="activeSpeakerIDs.has(key)"
              :data="cachedMastoData[key]"
            ></Participant>
          </template>
        </v-row>
      </v-card-text>
      <v-divider></v-divider>
      <v-card-actions class="justify-center" style="gap: 50px">
        <v-btn
          :icon="mdiMicrophoneQuestion"
          color="white"
          variant="flat"
        ></v-btn>
        <v-btn :icon="mdiPhoneRemove" color="red" variant="flat"></v-btn>
      </v-card-actions>
    </v-card>
  </main>
</template>
