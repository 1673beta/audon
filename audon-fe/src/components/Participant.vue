<script>
import { mdiMicrophone, mdiMicrophoneOff } from "@mdi/js";
import { webfinger } from "../assets/utils";
export default {
  setup() {
    return {
      mdiMicrophone,
      mdiMicrophoneOff,
      webfinger,
    };
  },
  props: {
    talking: Boolean,
    type: String,
    data: Object,
    muted: Boolean,
    emoji: String,
  },
  computed: {
    showEmoji() {
      return this.emoji !== undefined;
    },
    canSpeak() {
      return (
        this.type === "host" ||
        this.type === "cohost" ||
        this.type === "speaker"
      );
    },
    badgeProps() {
      switch (this.type) {
        case "host":
          return {
            content: "Host",
            colour: "deep-orange",
          };
        case "cohost":
          return {
            content: "Cohost",
            colour: "indigo",
          };
        case "speaker":
          return {
            content: "Speaker",
            colour: "",
          };
        default:
          return {
            content: "",
            colour: "",
          };
      }
    },
  },
};
</script>

<template>
  <v-col sm="3" cols="4" class="text-center">
    <v-badge
      v-if="canSpeak"
      :content="badgeProps.content"
      location="top"
      :color="badgeProps.colour"
    >
      <v-avatar :class="{ rounded: true, talk: talking }" size="70">
        <v-overlay
          v-model="showEmoji"
          contained
          persistent
          scroll-strategy="none"
          no-click-animation
          scrim="#000000"
          class="align-center justify-center reaction"
        >
          <span>{{ emoji }}</span>
        </v-overlay>
        <v-img :src="data?.avatar"></v-img>
      </v-avatar>
    </v-badge>
    <v-avatar
      v-else
      :class="{ rounded: true, talk: talking, 'mt-2': true }"
      size="70"
    >
      <v-overlay
        v-model="showEmoji"
        contained
        persistent
        scroll-strategy="none"
        no-click-animation
        scrim="#000000"
        class="align-center justify-center reaction"
      >
        <span>{{ emoji }}</span>
      </v-overlay>
      <v-img :src="data?.avatar"></v-img>
    </v-avatar>
    <h4 :class="canSpeak ? 'mt-1' : 'mt-2'">
      <v-icon
        v-if="canSpeak"
        :icon="muted ? mdiMicrophoneOff : mdiMicrophone"
      ></v-icon>
      <a :href="data?.url" class="plain" target="_blank">{{
        data?.displayName ?? webfinger(data)
      }}</a>
    </h4>
  </v-col>
</template>

<style scoped>
.talk {
  outline: 3px solid cornflowerblue;
}

.reaction span {
  font-size: 2rem;
  color: white;
  text-align: center;
}
</style>
