<script>
import { RouterLink } from "vue-router";
import { useMastodonStore } from "../stores/mastodon";

export default {
  setup() {
    return {
      donStore: useMastodonStore(),
    };
  },
  data() {
    return {
      query: ""
    }
  }
};
</script>

<template>
  <main>
    <div class="text-center my-10" >
      <v-avatar class="rounded" size="100">
        <v-img
          :src="donStore.userinfo?.avatar"
          :alt="donStore.userinfo?.displayName"
        >
        </v-img>
      </v-avatar>
      <h2 class="mt-5">
        {{ donStore.userinfo?.displayName }}
      </h2>
      <div>
        <a :href="donStore.userinfo?.url">{{ donStore.myWebfinger }}</a>
      </div>
    </div>
    <v-row class="text-center" justify="center">
      <!-- <v-col cols="12">
        <v-text-field v-mode="query"></v-text-field>
      </v-col> -->
      <v-col cols="12">
        <v-btn block :to="{name: 'create'}" color="indigo">部屋を作成</v-btn>
      </v-col>
    </v-row>
  </main>
</template>
