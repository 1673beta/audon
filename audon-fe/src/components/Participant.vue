<script>
import { mdiMicrophone, mdiMicrophoneOff } from "@mdi/js";
export default {
  props: {
    talking: Boolean,
    type: String,
    data: Object,
    muted: Boolean,
  },
  data () {
    return {
      mdiMicrophone,
      mdiMicrophoneOff
    }
  },
  computed: {
    canSpeak() {
      return this.type === "host" || this.type === "cohost" || this.type === "speaker";
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
            colour: ""
          }
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
        <v-img :src="data?.avatar"></v-img>
      </v-avatar>
    </v-badge>
    <v-avatar
      v-else
      :class="{ rounded: true, talk: talking, 'mt-2': true }"
      size="70"
    >
      <v-img :src="data?.avatar"></v-img>
    </v-avatar>
    <h4 :class="canSpeak ? 'mt-1' : 'mt-2'">
      <v-icon v-if="canSpeak" :icon="muted ? mdiMicrophoneOff : mdiMicrophone"></v-icon>
      <a :href="data?.url" target="_blank">{{ data?.displayName }}</a>
    </h4>
  </v-col>
</template>

<style scoped>
.talk {
  outline: 3px solid cornflowerblue;
}
a {
  color: inherit;
  text-decoration: inherit;
}
</style>
