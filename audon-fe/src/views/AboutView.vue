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
    準備中
    <div>
      <RouterLink :to="{ name: 'home' }">Home</RouterLink>
    </div>
  </div>
</template>

<style>
@media (min-width: 1024px) {
  .about {
    min-height: 100vh;
    display: flex;
    align-items: center;
  }
}
</style>
