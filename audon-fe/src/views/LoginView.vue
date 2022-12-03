<script>
import { RouterLink } from "vue-router";

export default {
  data() {
    return {
      server: "",
    };
  },
  methods: {
    submit() {
      console.log("submit called")
    }
  }
};
</script>

<template>
  <h1>Audon</h1>
  <v-form class="my-3">
    <v-text-field
      v-model="server"
      name="server"
      label="サーバーURL"
      hint="Mastodon / Pleroma / Misskey"
      placeholder="mastodon.example"
      class="mb-2"
      clearable
    />
    <v-btn block @click="submit">ログイン</v-btn>
  </v-form>
  <div class="w-100 text-right">
    <RouterLink to="/about">利用規約</RouterLink>
  </div>
</template>
