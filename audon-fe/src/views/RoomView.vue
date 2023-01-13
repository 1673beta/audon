<script>
import axios from "axios";
import { pushNotFound, webfinger } from "../assets/utils";
import { useMastodonStore } from "../stores/mastodon";
import { map, some, omit, filter, trim, clone } from "lodash-es";
import { darkTheme } from "picmo";
import { createPopup } from "@picmo/popup-picker";
import Participant from "../components/Participant.vue";
import {
  mdiMicrophone,
  mdiMicrophoneOff,
  mdiMicrophoneQuestion,
  mdiVolumeOff,
  mdiClose,
  mdiCheck,
  mdiAccountVoice,
  mdiLogout,
  mdiDotsVertical,
  mdiPencil,
  mdiEmoticon,
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
import { useVuelidate } from "@vuelidate/core";
import { helpers, maxLength, required } from "@vuelidate/validators";
import NoSleep from "@uriopass/nosleep.js";
import { DateTime } from "luxon";

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
    const noSleep = new NoSleep();
    document.addEventListener(
      "click",
      function enableNoSleep() {
        document.removeEventListener("click", enableNoSleep, false);
        noSleep.enable();
      },
      false
    );
    return {
      webfinger,
      clone,
      mdiLogout,
      mdiAccountVoice,
      mdiMicrophone,
      mdiMicrophoneOff,
      mdiMicrophoneQuestion,
      mdiVolumeOff,
      mdiClose,
      mdiCheck,
      mdiDotsVertical,
      mdiPencil,
      mdiEmoticon,
      v$: useVuelidate(),
      donStore: useMastodonStore(),
      decoder: new TextDecoder(),
      encoder: new TextEncoder(),
      roomClient: new Room(),
      emojiPicker: null,
    };
  },
  components: {
    Participant,
  },
  validations() {
    return {
      editingRoomInfo: {
        title: {
          required: helpers.withMessage(
            this.$t("form.titleRequired"),
            required
          ),
          maxLength: maxLength(100),
        },
        description: {
          maxLength: maxLength(500),
        },
      },
    };
  },
  data() {
    return {
      roomID: this.$route.params.id,
      loading: true,
      mainHeight: 700,
      roomInfo: {
        title: this.$t("connecting"),
        description: "",
        restriction: "",
        host: null,
        cohosts: [],
        speakers: [],
        created_at: null,
      },
      editingRoomInfo: {
        title: "",
        description: "",
        restriction: "",
      },
      relOptions: [
        { title: this.$t("form.relationships.everyone"), value: "everyone" },
        { title: this.$t("form.relationships.following"), value: "following" },
        { title: this.$t("form.relationships.follower"), value: "follower" },
        { title: this.$t("form.relationships.knowing"), value: "knowing" },
        { title: this.$t("form.relationships.mutual"), value: "mutual" },
        { title: this.$t("form.relationships.private"), value: "private" },
      ],
      participants: {},
      emojiReactions: {},
      cachedMastoData: {},
      activeSpeakerIDs: new Set(),
      mutedSpeakerIDs: new Set(),
      micGranted: false,
      autoplayDisabled: false,
      speakRequests: new Set(),
      showRequestNotification: false,
      showRequestDialog: false,
      showRequestedNotification: false,
      isEditLoading: false,
      showEditDialog: false,
      timeElapsed: "",
      preview: false,
    };
  },
  async created() {
    this.onResize();
    // fetch mastodon token
    if (!this.donStore.client || !this.donStore.authorized) {
      try {
        await this.donStore.fetchToken();
      } catch {
        this.preview = true;
        try {
          const resp = await axios.get(`/app/preview/${this.roomID}`);
          this.roomInfo = resp.data.roomInfo;
          this.participants = resp.data.participants;
          this.mutedSpeakerIDs = new Set(Object.keys(this.participants));
          for (const [key, value] of Object.entries(this.participants)) {
            if (value !== null) {
              this.fetchMastoData(key, value);
            }
          }
        } catch (error) {
          let query = { l: `/r/${this.roomID}` };
          switch (error.response?.status) {
            case 404:
              pushNotFound(this.$route);
              break;
            case 403:
              alert(this.$t("loginRequired"));
              break;
            case 410:
              alert(this.$t("errors.alreadyClosed"));
              query = undefined;
              break;
          }
          this.$router.push({
            name: "login",
            query,
          });
        } finally {
          this.loading = false;
        }
      }
    }
    if (!this.preview) {
      try {
        await this.joinRoom();
      } finally {
        this.loading = false;
      }
    }
    setInterval(this.refreshRemoteMuteStatus, 100);
    setInterval(this.refreshTimeElapsed, 1000);
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
    titleErrors() {
      const errors = this.v$.editingRoomInfo.title.$errors;
      const messages = map(errors, (e) => e.$message);
      return messages;
    },
  },
  methods: {
    refreshTimeElapsed() {
      if (!this.roomInfo.created_at) return;
      const now = DateTime.utc();
      const createdAt = DateTime.fromISO(this.roomInfo.created_at);
      const delta = now.diff(createdAt);
      this.timeElapsed = delta.toFormat("hh:mm:ss");
    },
    async joinRoom() {
      if (!this.donStore.authorized) return;
      try {
        const resp = await axios.get(`/api/room/${this.roomID}`);
        const room = new Room({
          // adaptiveStream: false,
          // dynacast: false,
          // publishDefaults: {
          //   stopMicTrackOnMute: false,
          //   simulcast: false,
          // },
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
          .on(RoomEvent.TrackMuted, (publication, participant) => {})
          .on(RoomEvent.TrackUnmuted, (publication, participant) => {})
          .on(RoomEvent.ParticipantDisconnected, (participant) => {
            self.participants = omit(self.participants, participant.identity);
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
                message = self.$t("roomEvent.closedByHost");
                break;
              case DisconnectReason.PARTICIPANT_REMOVED:
                message = self.$t("roomEvent.removed");
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
              { "kind": "emoji", "emoji": "..." }
              */
              const strData = self.decoder.decode(payload);
              const jsonData = JSON.parse(strData);
              const metadata = JSON.parse(participant.metadata);
              switch (jsonData?.kind) {
                case "emoji":
                  self.addEmojiReaction(participant.identity, jsonData.emoji);
                  break;
                case "speak_request": // someone is wanting to be a speaker
                  self.onSpeakRequestReceived(participant);
                  break;
                case "request_declined":
                  if (
                    self.isHost(participant.identity) ||
                    self.isCohost(metadata)
                  ) {
                    self.speakRequests.delete(jsonData.audon_id);
                    if (self.speakRequests.size < 1)
                      self.showRequestNotification = false;
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
            self.editingRoomInfo = clone(self.roomInfo);
            if (!self.roomInfo.speakers) return;
            for (const speakers of self.roomInfo.speakers) {
              self.speakRequests.delete(speakers.audon_id);
              if (self.speakRequests.size < 1)
                self.showRequestNotification = false;
            }
            if (self.iamSpeaker && !self.micGranted) {
              self.roomClient.localParticipant
                .setMicrophoneEnabled(true, captureOpts, publishOpts)
                .then((v) => {
                  self.micGranted = true;
                })
                .finally(() => {
                  self.roomClient.localParticipant.setMicrophoneEnabled(false);
                });
            }
          });
        await room.connect(resp.data.url, resp.data.token);
        this.roomClient = room;
        this.roomInfo = JSON.parse(room.metadata);
        this.editingRoomInfo = clone(this.roomInfo);
        this.addParticipant(room.localParticipant);
        for (const part of room.participants.values()) {
          this.addParticipant(part);
        }
        this.mutedSpeakerIDs.add(this.donStore.oauth.audon_id);
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
            alert(this.$t("microphoneBlocked"));
          } finally {
            await room.localParticipant.setMicrophoneEnabled(false);
          }
        }
      } catch (error) {
        switch (error.response?.status) {
          case 403:
            let message = "";
            switch (error.response?.data) {
              case "following":
                message = this.$t("errors.restriction.following");
                break;
              case "follower":
                message = this.$t("errors.restriction.follower");
                break;
              case "knowing":
                message = this.$t("errors.restriction.knowing");
                break;
              case "mutual":
                message = this.$t("errors.restriction.mutual");
                break;
              case "private":
                message = this.$t("errors.restriction.private");
                break;
              default:
                message = this.$t("errors.restriction.default");
            }
            alert(message);
            break;
          case 404:
            pushNotFound(this.$route);
            break;
          case 406:
            alert(this.$t("errors.alreadyConnected"));
            break;
          case 410:
            alert(this.$t("errors.alreadyClosed"));
            break;
          default:
            alert(error);
        }
        this.$router.push({ name: "home" });
      }
    },
    refreshRemoteMuteStatus() {
      for (const part of this.roomClient.participants.values()) {
        const track = part.getTrack(Track.Source.Microphone);
        if (track?.isMuted === false) {
          this.mutedSpeakerIDs.delete(part.identity);
        } else {
          this.mutedSpeakerIDs.add(part.identity);
        }
      }
    },
    onResize() {
      const mainArea = document.getElementById("mainArea");
      const height = mainArea.clientHeight;
      this.mainHeight = height > 700 ? 700 : window.innerHeight - 95;
    },
    isHost(identity) {
      return identity === this.roomInfo.host?.audon_id;
    },
    isCohost(metadata) {
      return (
        metadata &&
        some(this.roomInfo.cohosts, {
          remote_id: metadata.remote_id,
          remote_url: metadata.remote_url,
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
      if (confirm(this.$t("speakRequest.dialog"))) {
        await this.publishDataToHostAndCohosts({ kind: "speak_request" });
        this.showRequestedNotification = true;
      }
    },
    onPickerPopup() {
      const btn = document.getElementById("pickerButton");
      if (!this.emojiPicker) {
        const picker = createPopup(
          {
            theme: darkTheme,
            emojiSize: "1.8rem",
            autoFocus: "none",
          },
          {
            referenceElement: btn,
            triggerElement: btn,
            position: "top",
            hideOnEmojiSelect: true,
          }
        );
        const self = this;
        picker.addEventListener("emoji:select", ({ emoji }) => {
          self.onEmojiSelected(emoji);
        });
        this.emojiPicker = picker;
      }
      this.emojiPicker.open();
    },
    async onEmojiSelected(emoji) {
      this.showEmojiMenu = false;
      const data = { kind: "emoji", emoji };
      const payload = this.encoder.encode(JSON.stringify(data));
      await this.roomClient.localParticipant.publishData(
        payload,
        DataPacket_Kind.RELIABLE
      );
      this.addEmojiReaction(this.roomClient.localParticipant.identity, emoji);
    },
    addEmojiReaction(identity, emoji) {
      const self = this;
      if (self.emojiReactions[identity]) {
        clearTimeout(self.emojiReactions[identity].timeoutID);
      }
      const timeoutID = setTimeout(() => {
        self.emojiReactions = omit(self.emojiReactions, identity);
      }, 5000);
      self.emojiReactions[identity] = {
        timeoutID,
        emoji,
      };
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
      const myIdentity = this.roomClient.localParticipant.identity;
      if (this.iamHost || this.iamCohost || this.iamSpeaker) {
        try {
          let newMicStatus = false;
          if (!this.micGranted) {
            newMicStatus = true;
            await this.roomClient.localParticipant.setMicrophoneEnabled(
              newMicStatus,
              captureOpts,
              publishOpts
            );
          } else if (myTrack) {
            newMicStatus = myTrack.isMuted;
            await this.roomClient.localParticipant.setMicrophoneEnabled(
              newMicStatus,
              captureOpts,
              publishOpts
            );
          }
          if (newMicStatus) {
            this.mutedSpeakerIDs.delete(myIdentity);
          } else {
            this.mutedSpeakerIDs.add(myIdentity);
          }
        } catch {
          alert(this.$t("microphoneBlocked"));
        }
      } else {
        this.requestSpeak();
      }
    },
    async onRoomClose() {
      // TODO: change this from confirm to a vuetify thing
      if (confirm(this.$t("closeRoomConfirm"))) {
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
        alert(this.$t("errors.connectionFailed"));
        await this.roomClient.disconnect();
      }
    },
    async onEditSubmit() {
      this.editingRoomInfo.title = trim(this.editingRoomInfo.title);
      this.editingRoomInfo.description = trim(this.editingRoomInfo.description);
      const isFormCorrect = await this.v$.$validate();
      if (!isFormCorrect) {
        return;
      }

      this.isEditLoading = true;

      try {
        const payload = {
          title: this.editingRoomInfo.title,
          description: this.editingRoomInfo.description,
          restriction: this.editingRoomInfo.restriction,
        };
        await axios.patch(`/api/room/${this.roomID}`, payload);
      } catch (error) {
        alert(error);
      } finally {
        this.isEditLoading = false;
        this.showEditDialog = false;
      }
    },
  },
};
</script>

<template>
  <v-dialog v-model="showEditDialog" max-width="500" persistent>
    <v-card :loading="isEditLoading">
      <v-card-title>{{ $t("editRoom") }}</v-card-title>
      <v-card-text>
        <v-text-field
          v-model="editingRoomInfo.title"
          :label="$t('form.title')"
          :error-messages="titleErrors"
          :counter="100"
          required
          @input="v$.editingRoomInfo.title.$touch()"
          @blur="v$.editingRoomInfo.title.$touch()"
        ></v-text-field>
        <v-textarea
          auto-grow
          v-model="editingRoomInfo.description"
          rows="2"
          :label="$t('form.description')"
          :counter="500"
        ></v-textarea>
        <v-select
          :items="relOptions"
          :label="$t('form.restriction')"
          v-model="editingRoomInfo.restriction"
          :messages="[$t('form.cohostCanAlwaysJoin')]"
        ></v-select>
      </v-card-text>
      <v-divider></v-divider>
      <v-card-actions class="justify-end">
        <v-btn
          @click="
            showEditDialog = false;
            editingRoomInfo = clone(roomInfo);
          "
          >{{ $t("cancel") }}</v-btn
        >
        <v-btn :disabled="isEditLoading" @click="onEditSubmit">{{
          $t("save")
        }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
  <v-dialog v-model="autoplayDisabled" max-width="500" persistent>
    <v-alert color="indigo">
      <div class="mb-5">
        {{ $t("browserMuted") }}
      </div>
      <div class="text-center mb-3">
        <v-btn color="gray" @click="onStartListening">{{
          $t("startListening")
        }}</v-btn>
      </div>
      <div class="text-center">
        <v-btn variant="text" @click="roomClient.disconnect()">{{
          $t("leaveRoom")
        }}</v-btn>
      </div>
    </v-alert>
  </v-dialog>
  <v-dialog v-model="showRequestDialog" max-width="500">
    <v-card max-height="600" class="d-flex flex-column">
      <v-card-title>{{ $t("speakRequest.label") }}</v-card-title>
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
                class="text-body plain"
                target="_blank"
                >{{ webfinger(cachedMastoData[id]) }}</a
              >
            </v-list-item-subtitle>
          </v-list-item>
        </v-list>
        <p class="text-center py-3" v-else>
          {{ $t("speakRequest.norequest") }}
        </p>
      </v-card-text>
      <v-divider></v-divider>
      <v-card-actions class="justify-end">
        <v-btn @click="showRequestDialog = false">{{ $t("close") }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
  <v-snackbar
    location="top"
    :timeout="5000"
    v-model="showRequestedNotification"
    color="info"
  >
    <strong>{{ $t("speakRequest.sent") }}</strong>
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
      <strong>{{ $t("speakRequest.receive") }}</strong>
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
      <v-card-title class="d-flex align-center">
        <div class="mr-auto overflow-y-auto">{{ roomInfo.title }}</div>
        <v-chip v-if="timeElapsed" class="mx-1 flex-shrink-0">
          <code>{{ timeElapsed }}</code>
        </v-chip>
        <div v-if="iamHost" class="flex-shrink-0">
          <v-btn
            size="small"
            variant="text"
            color="white"
            :icon="mdiPencil"
            @click="showEditDialog = true"
          ></v-btn>
        </div>
      </v-card-title>
      <div
        class="overflow-auto flex-shrink-0 pb-2"
        v-if="roomInfo.description"
        style="height: 100px"
      >
        <v-container class="py-0">
          <p style="white-space: pre-wrap">{{ roomInfo.description }}</p>
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
              :emoji="emojiReactions[key]?.emoji"
            ></Participant>
            <Participant
              v-if="isCohost(value)"
              :talking="isTalking(key)"
              type="cohost"
              :data="cachedMastoData[key]"
              :muted="mutedSpeakerIDs.has(key)"
              :emoji="emojiReactions[key]?.emoji"
            ></Participant>
            <Participant
              v-if="isSpeaker(key)"
              :talking="isTalking(key)"
              type="speaker"
              :data="cachedMastoData[key]"
              :muted="mutedSpeakerIDs.has(key)"
              :emoji="emojiReactions[key]?.emoji"
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
              :emoji="emojiReactions[key]?.emoji"
            ></Participant>
          </template>
        </v-row>
      </v-card-text>
      <v-divider></v-divider>
      <v-card-actions v-if="preview" class="justify-center">
        <v-btn
          variant="flat"
          color="indigo"
          block
          :to="{ name: 'login', query: { l: `/r/${roomID}` } }"
          >{{ $t("enterRoom") }}</v-btn
        >
      </v-card-actions>
      <v-card-actions v-else class="justify-center" style="gap: 20px">
        <v-btn
          :icon="mdiEmoticon"
          color="white"
          variant="flat"
          @click="onPickerPopup"
          id="pickerButton"
        >
        </v-btn>
        <v-btn
          :icon="micStatusIcon"
          color="white"
          variant="flat"
          @click="onToggleMute"
        ></v-btn>
        <v-btn
          v-if="iamHost"
          :icon="mdiLogout"
          color="red"
          @click="onRoomClose"
          variant="flat"
        ></v-btn>
        <v-btn
          v-else
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

<style scoped></style>
