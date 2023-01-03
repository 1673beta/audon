<script>
import { RouterLink } from "vue-router";
import {useMastodonStore} from "../stores/mastodon"

export default {
  setup() {
    const donStore = useMastodonStore();

    return {
      donStore
    }
  },
  mounted() {
  }
};
</script>

<template>
  <div class="about">
    <div>
      Under construction
    </div>
  </div>
</template>

<style scoped>
</style>
