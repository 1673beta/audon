<script>
import axios from "axios";
import { pushNotFound, webfinger } from "../assets/utils";
import { useMastodonStore } from "../stores/mastodon";
import { map, some, omit, filter } from "lodash-es";
import Participant from "../components/Participant.vue";
import {
  mdiMicrophone,
  mdiMicrophoneOff,
  mdiMicrophoneQuestion,
  mdiDoorClosed,
  mdiVolumeOff,
  mdiClose,
  mdiCheck,
  mdiAccountVoice,
  mdiLogout
} from "@mdi/js";
import {
  Room,
  RoomEvent,
  Track,
  DisconnectReason,
  DataPacket_Kind,
  AudioPresets,
} from "livekit-client";
import { login } from "masto";

const publishOpts = {
  audioBitrate: AudioPresets.music,
  // forceStereo: true,
};

const captureOpts = {
  // autoGainControl: false,
  // echoCancellation: false,
  // sampleRate: 48000,
  // sampleSize: 16,
  // channelCount: 2
};

export default {
  setup() {
    return {
      donStore: useMastodonStore(),
      decoder: new TextDecoder(),
      encoder: new TextEncoder(),
    };
  },
  components: {
    Participant,
  },
  data() {
    return {
      mdiLogout,
      mdiAccountVoice,
      mdiMicrophone,
      mdiMicrophoneOff,
      mdiMicrophoneQuestion,
      mdiDoorClosed,
      mdiVolumeOff,
      mdiClose,
      mdiCheck,
      roomID: this.$route.params.id,
      loading: false,
      mainHeight: 600,
      roomClient: new Room(),
      roomInfo: {
        title: "",
        description: "",
        host: null,
        cohosts: [],
        speakers: [],
        createdAt: null,
      },
      participants: {},
      cachedMastoData: {},
      activeSpeakerIDs: new Set(),
      mutedSpeakerIDs: new Set(),
      micGranted: false,
      autoplayDisabled: false,
      speakRequests: new Set(),
      showRequestNotification: false,
      showRequestDialog: false,
      showRequestedNotification: false,
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
        (this.iamHost || this.iamCohost || this.iamSpeaker) &&
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

      return this.isCohost({ remote_id: myInfo.id, remote_url: myInfo.url });
    },
    iamSpeaker() {
      const myAudonID = this.donStore.oauth.audon_id;
      if (!myAudonID) return false;

      return this.isSpeaker(myAudonID);
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
    webfinger,
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
              self.autoplayDisabled = true;
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
          })
          .on(RoomEvent.DataReceived, (payload, participant, kind) => {
            try {
              /* data should be like
              { "kind": "speak_request" }
              { "kind": "chat", "data": "..." }
              { "kind": "request_declined", "audon_id": "..."}
              */
              const strData = self.decoder.decode(payload);
              const jsonData = JSON.parse(strData);
              const metadata = JSON.parse(participant.metadata);
              switch (jsonData?.kind) {
                case "speak_request": // someone is wanting to be a speaker
                  self.onSpeakRequestReceived(participant);
                  break;
                case "request_declined":
                  if (
                    self.isHost(participant.identity) ||
                    self.isCohost(metadata)
                  ) {
                    self.speakRequests.delete(jsonData.audon_id);
                  }
                  break;
              }
            } catch (error) {
              console.log(
                "invalida data received from: ",
                participant.identity
              );
            }
          })
          .on(RoomEvent.RoomMetadataChanged, (metadata) => {
            self.roomInfo = JSON.parse(metadata);
            for (const speakers of self.roomInfo.speakers) {
              self.speakRequests.delete(speakers.audon_id);
            }
            if (self.iamSpeaker || !self.micGranted) {
              self.roomClient.localParticipant
                .setMicrophoneEnabled(true, captureOpts, publishOpts)
                .then((v) => {
                  self.micGranted = true;
                });
            }
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
        if (this.iamHost || this.iamCohost || this.iamSpeaker) {
          try {
            await room.localParticipant.setMicrophoneEnabled(
              true,
              captureOpts,
              publishOpts
            );
          } catch {
            alert("ブラウザが録音を許可していません");
          }
        }
      } catch (error) {
        switch (error.response?.status) {
          case 404:
            pushNotFound(this.$route);
            break;
          case 406:
            alert(
              "他のデバイスで入室済みです。切断された場合はしばらく待ってからやり直してください。"
            );
            this.$router.push({ name: "home" });
            break;
          case 410:
            alert("この部屋はすでに閉じられています。");
            this.$router.push({ name: "home" });
            break;
          default:
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
    isSpeaker(identity) {
      return identity && some(this.roomInfo.speakers, { audon_id: identity });
    },
    isTalking(identity) {
      return (
        this.activeSpeakerIDs.has(identity) &&
        !this.mutedSpeakerIDs.has(identity)
      );
    },
    onSpeakRequestReceived(participant) {
      if (this.iamHost || this.iamCohost) {
        if (this.speakRequests.has(participant.identity)) return;
        this.speakRequests.add(participant.identity);
        this.showRequestNotification = true;
      }
    },
    async onAcceptRequest(identity) {
      // promote user to a speaker
      // the livekit server will update room metadata
      try {
        await axios.put(`/api/room/${this.roomID}/${identity}`);
      } catch (reqError) {
        console.log("permission update request error: ", reqError);
      }
    },
    async onDeclineRequest(identity) {
      // share declined identity with host and other cohosts
      if (!this.speakRequests.delete(identity)) return;
      const data = { kind: "request_declined", audon_id: identity };
      await this.publishDataToHostAndCohosts(data);
    },
    async requestSpeak() {
      if (confirm("発言をリクエストしますか？")) {
        await this.publishDataToHostAndCohosts({ kind: "speak_request" });
        this.showRequestedNotification = true;
      }
    },
    async publishDataToHostAndCohosts(data) {
      const payload = this.encoder.encode(JSON.stringify(data));
      // participants - speakers
      const hostandcohosts = filter(
        Array.from(this.roomClient.participants.values()),
        (p) => {
          const metadata = JSON.parse(p.metadata);
          return this.isHost(p.identity) || this.isCohost(metadata);
        }
      );
      await this.roomClient.localParticipant.publishData(
        payload,
        DataPacket_Kind.RELIABLE,
        hostandcohosts
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
      if (this.iamHost || this.iamCohost || this.iamSpeaker) {
        try {
          if (!this.micGranted) {
            await this.roomClient.localParticipant.setMicrophoneEnabled(
              true,
              captureOpts,
              publishOpts
            );
          } else if (myTrack) {
            await this.roomClient.localParticipant.setMicrophoneEnabled(
              myTrack.isMuted,
              captureOpts,
              publishOpts
            );
          }
        } catch {
          alert("ブラウザが録音を許可していません");
        }
      } else {
        // alert("リクエストはアップデートで実装予定です！");
        this.requestSpeak();
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
    },
  },
};
</script>

<template>
  <v-dialog v-model="autoplayDisabled" max-width="500" persistent>
    <v-alert color="indigo">
      <div class="mb-5">
        ブラウザの設定により無音になっています。続行するには「視聴を始める」ボタンを押してください。
      </div>
      <div class="text-center mb-3">
        <v-btn color="gray" @click="onStartListening">視聴を始める</v-btn>
      </div>
      <div class="text-center">
        <v-btn variant="text" @click="roomClient.disconnect()">退室する</v-btn>
      </div>
    </v-alert>
  </v-dialog>
  <v-dialog v-model="showRequestDialog" max-width="500">
    <v-card max-height="600" class="d-flex flex-column">
      <v-card-title>発言リクエスト</v-card-title>
      <v-card-text class="flex-grow-1 overflow-auto py-0">
        <v-list v-if="speakRequests.size > 0" lines="two" variant="tonal">
          <v-list-item
            v-for="id of Array.from(speakRequests)"
            :key="id"
            :title="cachedMastoData[id]?.displayName"
            class="my-1"
            rounded
          >
            <template v-slot:prepend>
              <v-avatar class="rounded">
                <v-img :src="cachedMastoData[id]?.avatar"></v-img>
              </v-avatar>
            </template>
            <template v-slot:append>
              <v-btn
                class="mr-2"
                size="small"
                variant="text"
                :icon="mdiCheck"
                @click="onAcceptRequest(id)"
              ></v-btn>
              <v-btn
                size="small"
                variant="text"
                :icon="mdiClose"
                @click="onDeclineRequest(id)"
              ></v-btn>
            </template>
            <v-list-item-subtitle>
              <a
                :href="cachedMastoData[id]?.url"
                class="text-body"
                style="text-decoration: inherit; color: inherit"
                target="_blank"
                >{{ webfinger(cachedMastoData[id]) }}</a
              >
            </v-list-item-subtitle>
          </v-list-item>
        </v-list>
        <p class="text-center py-3" v-else>リクエストはありません</p>
      </v-card-text>
      <v-divider></v-divider>
      <v-card-actions class="justify-end">
        <v-btn @click="showRequestDialog = false">閉じる</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
  <v-snackbar
    location="top"
    :timeout="5000"
    v-model="showRequestedNotification"
    color="info"
  >
    <strong>発言リクエストを送信しました！</strong>
    <template v-slot:actions>
      <v-btn
        variant="text"
        @click="showRequestedNotification = false"
        :icon="mdiClose"
        size="small"
      ></v-btn>
    </template>
  </v-snackbar>
  <v-snackbar
    location="top"
    :timeout="-1"
    v-model="showRequestNotification"
    color="info"
  >
    <div
      style="cursor: pointer"
      @click="
        showRequestDialog = true;
        showRequestNotification = false;
      "
    >
      <strong>新しい発言リクエストがあります</strong>
    </div>
    <template v-slot:actions>
      <v-btn
        variant="text"
        @click="showRequestNotification = false"
        :icon="mdiClose"
        size="small"
      ></v-btn>
    </template>
  </v-snackbar>
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
          <p style="white-space: pre-wrap;">{{ roomInfo.description }}</p>
        </v-container>
      </div>
      <v-divider></v-divider>
      <v-card-text class="flex-grow-1 overflow-auto">
        <v-row justify="start">
          <template v-for="(value, key) of participants" :key="key">
            <Participant
              v-if="isHost(key)"
              :talking="isTalking(key)"
              type="host"
              :data="cachedMastoData[key]"
              :muted="mutedSpeakerIDs.has(key)"
            ></Participant>
            <Participant
              v-if="isCohost(value)"
              :talking="isTalking(key)"
              type="cohost"
              :data="cachedMastoData[key]"
              :muted="mutedSpeakerIDs.has(key)"
            ></Participant>
            <Participant
              v-if="isSpeaker(key)"
              :talking="isTalking(key)"
              type="speaker"
              :data="cachedMastoData[key]"
              :muted="mutedSpeakerIDs.has(key)"
            >
            </Participant>
          </template>
        </v-row>
        <v-row justify="start">
          <template v-for="(value, key) of participants" :key="key">
            <Participant
              v-if="!isHost(key) && !isCohost(value) && !isSpeaker(key)"
              :data="cachedMastoData[key]"
              type="listener"
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
          :icon="mdiLogout"
          color="red"
          @click="roomClient.disconnect()"
          variant="flat"
        ></v-btn>
        <v-badge
          v-if="iamHost || iamCohost"
          color="info"
          :model-value="speakRequests.size > 0"
          :content="speakRequests.size"
        >
          <v-btn
            :icon="mdiAccountVoice"
            variant="flat"
            color="white"
            @click="
              showRequestDialog = true;
              showRequestNotification = false;
            "
          >
          </v-btn>
        </v-badge>
      </v-card-actions>
    </v-card>
  </main>
</template>
