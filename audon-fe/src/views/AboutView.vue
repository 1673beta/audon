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
    <h1>This is an about page</h1>
    <RouterLink :to="{ name: 'login' }">Login</RouterLink>
    <RouterLink :to="{ name: 'home' }">Home</RouterLink>
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
