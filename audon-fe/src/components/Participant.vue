<!-- eslint-disable vue/multi-word-component-names -->
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
    preview: Boolean,
    enableMenu: Boolean,
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
            content: this.$t("role.host"),
            colour: "deep-orange",
          };
        case "cohost":
          return {
            content: this.$t("role.cohost"),
            colour: "indigo",
          };
        case "speaker":
          return {
            content: this.$t("role.speaker"),
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
          <div class="d-flex align-center justify-center">
            <img class="emoji" :src="emoji" />
          </div>
        </v-overlay>
        <v-img
          :class="{ cursorPointer: enableMenu }"
          :id="`mod-${data?.identity}`"
          :src="data?.avatar"
        ></v-img>
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
        <div class="d-flex align-center justify-center">
          <img class="emoji" :src="emoji" />
        </div>
      </v-overlay>
      <v-img
        :class="{ cursorPointer: enableMenu }"
        :id="`mod-${data?.identity}`"
        :src="data?.avatar"
      ></v-img>
    </v-avatar>
    <v-menu v-if="enableMenu" :activator="`#mod-${data?.identity}`">
      <v-list>
        <v-list-item
          :title="$t('moderation.promote', { role: $t('role.cohost') })"
          @click="$emit('moderate', this.data?.identity, 'cohost')"
        ></v-list-item>
        <v-list-item
          v-if="type !== 'speaker'"
          :title="$t('moderation.promote', { role: $t('role.speaker') })"
          @click="$emit('moderate', this.data?.identity, 'speaker')"
        ></v-list-item>
        <v-list-item
          v-else
          :title="$t('moderation.demote')"
          @click="$emit('moderate', this.data?.identity, 'demote')"
        ></v-list-item>
        <v-list-item
          :title="$t('moderation.kick')"
          @click="$emit('moderate', this.data?.identity, 'kick')"
        ></v-list-item>
      </v-list>
    </v-menu>
    <h4 :class="canSpeak ? 'mt-1' : 'mt-2'">
      <v-icon
        v-if="canSpeak && !preview"
        :icon="muted ? mdiMicrophoneOff : mdiMicrophone"
      ></v-icon>
      <a :href="data?.url" class="plain" target="_blank">{{
        !data?.displayName ? webfinger(data) : data?.displayName
      }}</a>
    </h4>
  </v-col>
</template>

<style scoped>
.talk {
  outline: 3px solid cornflowerblue;
}

.reaction img {
  height: 2rem;
}

.cursorPointer {
  cursor: pointer;
}
</style>
