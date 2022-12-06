<script>
import { RouterLink } from "vue-router";
import axios from "axios";

export default {
  async mounted() {
    try {
      await axios.get("/app/verify");
    } catch {}
  },
};
</script>

<template>
  <div class="about">
    <h1>This is an about page</h1>
    <RouterLink to="login">Login</RouterLink>
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
