<script>
import { Room } from "livekit-client";
import { useMastodonStore } from "../stores/mastodon";
import { mdiArrowRightBold } from "@mdi/js";
import { pushNotFound } from "../assets/utils";
import axios from "axios";

export default {
  setup() {
    return {
      mdiArrowRightBold,
      donStore: useMastodonStore(),
    };
  },
  data() {
    return {
      roomID: this.$route.params.id,
      isLoading: true,
      roomToken: null,
      dialogEnabled: true,
      uploading: false,
    };
  },
  emits: ["connect"],
  props: {
    roomId: String,
    roomClient: Room,
  },
  computed: {
    uploadEnabled() {
      // return this.roomToken?.original && this.roomToken?.indicator;
      return false;
    },
  },
  async mounted() {
    try {
      await this.donStore.fetchToken();
      const resp = await axios.post(
        `/api/room/${this.roomID}`,
        this.donStore.userinfo
      );
      this.roomToken = resp.data;
    } catch (error) {
      this.dialogEnabled = false;
      if (error.response?.status === 401) {
        return;
      }
      let message = "";
      switch (error.response?.status) {
        case 403:
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
    } finally {
      this.isLoading = false;
    }
  },
  methods: {
    async joining(indicator) {
      try {
        this.donStore.avatar = this.roomToken.original;
        if (indicator && this.uploadEnabled) {
          this.uploading = true;
          try {
            await this.donStore.updateAvatar(this.roomToken.indicator);
          } finally {
            this.uploading = false;
            this.dialogEnabled = false;
            this.$emit("connect", this.roomToken);
          }
        } else {
          this.dialogEnabled = false;
          this.$emit("connect", this.roomToken);
        }
      } catch {
        alert(this.$t("errors.connectionFailed"));
      }
    },
  },
};
</script>

<template>
  <v-overlay
    :model-value="isLoading || uploading"
    persistent
    class="align-center justify-center"
  >
    <v-progress-circular indeterminate size="40"></v-progress-circular>
  </v-overlay>
  <v-dialog
    v-if="!isLoading"
    v-model="dialogEnabled"
    max-width="500"
    persistent
  >
    <v-alert v-if="uploadEnabled" color="deep-purple-darken-2">
      <div>
        {{ $t("onlineIndicator.message") }}
      </div>
      <div class="mt-3 text-center">
        <v-avatar class="rounded" size="80">
          <v-img :src="roomToken.indicator"></v-img>
        </v-avatar>
      </div>
      <v-alert
        class="mt-3"
        density="compact"
        type="success"
        color="white"
        variant="tonal"
      >
        {{ $t("onlineIndicator.hint") }}
      </v-alert>
      <v-alert
        class="mt-3"
        border="start"
        density="compact"
        type="warning"
        variant="outlined"
      >
        {{ $t("onlineIndicator.warning") }}
      </v-alert>
      <div class="mt-3 mb-1 d-flex align-center justify-space-around">
        <v-btn @click="joining(false)">{{ $t("onlineIndicator.nope") }}</v-btn>
        <v-btn color="indigo" @click="joining(true)">{{
          $t("onlineIndicator.sure")
        }}</v-btn>
      </div>
    </v-alert>
    <v-alert v-else color="indigo">
      <div class="mb-5">
        {{ $t("browserMuted") }}
      </div>
      <div class="text-center mb-1">
        <v-btn color="gray" @click="joining(false)">{{
          $t("startListening")
        }}</v-btn>
      </div>
    </v-alert>
  </v-dialog>
</template>
