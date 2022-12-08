<script>
export default {
  props: {
    talking: Boolean,
    type: String,
    data: Object,
  },
  computed: {
    isHostOrCohost() {
      return this.type === "host" || this.type === "cohost";
    },
    badgeProps() {
      switch (this.type) {
        case "host":
          return {
            content: "Host",
            colour: "primary",
          };
        case "cohost":
          return {
            content: "Cohost",
            colour: "secondary",
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
      v-if="isHostOrCohost"
      :content="badgeProps.content"
      location="top"
      :color="badgeProps.colour"
    >
      <v-avatar :class="{ rounded: true, talk: talking }" size="70">
        <v-img :src="data?.avatar"></v-img>
      </v-avatar>
    </v-badge>
    <v-avatar v-else :class="{ rounded: true, talk: talking, 'mt-2': true }" size="70">
      <v-img :src="data?.avatar"></v-img>
    </v-avatar>
    <h4 :class="isHostOrCohost ? 'mt-1' : 'mt-2'">{{ data?.displayName }}</h4>
  </v-col>
</template>

<style scoped>
.talk {
  outline: 3px solid cornflowerblue;
}
</style>
