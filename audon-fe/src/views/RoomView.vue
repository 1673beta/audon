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
  mdiDoorClosed,
  mdiVolumeOff
} from "@mdi/js";
import {
  Room,
  RoomEvent,
  Track,
  DisconnectReason,
} from "livekit-client";
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
      mdiDoorClosed,
      mdiVolumeOff,
      roomID: this.$route.params.id,
      loading: false,
      mainHeight: 600,
      roomClient: new Room(),
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
      mutedSpeakerIDs: new Set(),
      micGranted: false,
      autoplayDisabled: false,
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
  computed: {
    iamMuted() {
      const myAudonID = this.donStore.oauth.audon_id;
      return (
        (this.iamHost || this.iamCohost) &&
        this.micGranted &&
        this.mutedSpeakerIDs.has(myAudonID)
      );
    },
    iamHost() {
      const myAudonID = this.donStore.oauth.audon_id;
      if (!myAudonID) return false;

      return this.isHost(myAudonID);
    },
    iamCohost() {
      const myInfo = this.donStore.userinfo;
      if (!myInfo) return false;

      return this.isCohost({remote_id: myInfo.id , remote_url: myInfo.url});
    },
    micStatusIcon() {
      if (!this.micGranted) {
        return mdiMicrophoneQuestion;
      }
      if (this.iamMuted) {
        return mdiMicrophoneOff;
      }
      return mdiMicrophone;
    },
  },
  methods: {
    async joinRoom() {
      if (!this.donStore.authorized) return;
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
          .on(RoomEvent.LocalTrackPublished, (publication, participant) => {
            self.micGranted = true;
            self.mutedSpeakerIDs.delete(participant.identity);
          })
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
          .on(RoomEvent.TrackMuted, (publication, participant) => {
            self.mutedSpeakerIDs.add(participant.identity);
          })
          .on(RoomEvent.TrackUnmuted, (publication, participant) => {
            self.mutedSpeakerIDs.delete(participant.identity);
          })
          .on(RoomEvent.ParticipantDisconnected, (participant) => {
            self.participants = omit(self.participants, participant.identity);
            self.mutedSpeakerIDs.delete(participant.identity);
          })
          .on(RoomEvent.AudioPlaybackStatusChanged, () => {
            if (!room.canPlaybackAudio) {
              // FIXME: popup a dialog to ask user to allow audio playback
              // alert("autoplay not permitted");
              self.autoplayDisabled = true
            }
          })
          .on(RoomEvent.Disconnected, (reason) => {
            // TODO: change this from alert to a vuetify thing
            let message = "";
            switch (reason) {
              case DisconnectReason.ROOM_DELETED:
                message = "ホストにより部屋が閉じられました。";
                break;
              case DisconnectReason.PARTICIPANT_REMOVED:
                message = "部屋から退去しました";
                break;
              case DisconnectReason.CLIENT_INITIATED:
                break;
              default:
                message = "Disconnected due to unknown reasons";
            }
            if (message !== "") {
              alert(message);
            }
            self.$router.push({ name: "home" });
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
        if (this.iamHost || this.iamCohost) {
          try {
            await room.localParticipant.setMicrophoneEnabled(true);
          } catch {
            alert("ブラウザが録音を許可していません");
          }
        }
      } catch (error) {
        if (error.response?.status === 404) {
          pushNotFound(this.$route);
        } else if (error.response?.status === 406) {
          alert(
            "他のデバイスで入室済みです。切断された場合はしばらく待ってからやり直してください。"
          );
          this.$router.push({ name: "home" });
        } else {
          // FIXME: error handling
          alert(error);
          this.$router.push({ name: "home" });
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
    isHost(identity) {
      return identity === this.roomInfo.host?.audon_id;
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
      if (metadata) {
        this.participants[participant.identity] = metadata;
        const track = participant.getTrack(Track.Source.Microphone);
        if (
          (this.isHost(participant.identity) || this.isCohost(metadata)) &&
          track?.isMuted
        ) {
          this.mutedSpeakerIDs.add(participant.identity);
        }
      }
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
    async onToggleMute() {
      const myTrack = this.roomClient.localParticipant.getTrack(
        Track.Source.Microphone
      );
      if (this.iamHost || this.iamCohost) {
        try {
          if (!this.micGranted) {
            await this.roomClient.localParticipant.setMicrophoneEnabled(true);
          } else if (myTrack) {
            await this.roomClient.localParticipant.setMicrophoneEnabled(
              myTrack.isMuted
            );
          }
        } catch {
          alert("ブラウザが録音を許可していません");
        }
      } else {
        alert("リクエストはアップデートで実装予定です！");
      }
    },
    async onRoomClose() {
      // TODO: change this from confirm to a vuetify thing
      if (confirm("この部屋を閉じますか？")) {
        try {
          await axios.delete(`/api/room/${this.roomID}`);
        } catch (error) {
          alert(error);
        }
      }
    },
    async onStartListening() {
      try {
        await this.roomClient.startAudio();
        this.autoplayDisabled = false;
      } catch {
        alert("接続できませんでした。退室します。");
        await this.roomClient.disconnect();
      }
    }
  },
};
</script>

<template>
  <v-dialog v-model="autoplayDisabled" max-width="500" persistent>
    <v-alert color="indigo">
      <div class="mb-5">ブラウザの設定により無音になっています。続行するには「視聴を始める」ボタンを押してください。</div>
      <div class="text-center mb-3">
        <v-btn color="gray" @click="onStartListening">視聴を始める</v-btn>
      </div>
      <div class="text-center">
        <v-btn variant="text" @click="roomClient.disconnect()">退室する</v-btn>
      </div>
    </v-alert>
  </v-dialog>
  <div class="d-none" ref="audioDOM"></div>
  <main class="fill-height" v-resize="onResize">
    <v-card :height="mainHeight" :loading="loading" class="d-flex flex-column">
      <v-card-title class="d-flex justify-space-between">
        <div>{{ roomInfo.title }}</div>
        <div>
          <v-btn
            v-if="iamHost"
            :append-icon="mdiDoorClosed"
            variant="outlined"
            color="red"
            @click="onRoomClose"
          >
            閉室
          </v-btn>
        </div>
      </v-card-title>
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
              v-if="isHost(key)"
              :talking="activeSpeakerIDs.has(key)"
              type="host"
              :data="cachedMastoData[key]"
              :muted="mutedSpeakerIDs.has(key)"
            ></Participant>
            <Participant
              v-if="isCohost(value)"
              :talking="activeSpeakerIDs.has(key)"
              type="cohost"
              :data="cachedMastoData[key]"
              :muted="mutedSpeakerIDs.has(key)"
            ></Participant>
          </template>
        </v-row>
        <v-row>
          <template v-for="(value, key) of participants" :key="key">
            <Participant
              v-if="!isHost(key) && !isCohost(value)"
              :talking="activeSpeakerIDs.has(key)"
              :data="cachedMastoData[key]"
            ></Participant>
          </template>
        </v-row>
      </v-card-text>
      <v-divider></v-divider>
      <v-card-actions class="justify-center" style="gap: 50px">
        <v-btn
          :icon="micStatusIcon"
          color="white"
          variant="flat"
          @click="onToggleMute"
        ></v-btn>
        <v-btn
          :icon="mdiPhoneRemove"
          color="red"
          @click="roomClient.disconnect()"
          variant="flat"
        ></v-btn>
      </v-card-actions>
    </v-card>
  </main>
</template>
