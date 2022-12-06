<script setup>
import { RouterView } from 'vue-router'
import { useMastodonStore } from "./stores/mastodon"

const donStore = useMastodonStore();
donStore.fetchToken();
</script>

<template>
  <v-container class="fill-height">
    <v-row align="center" justify="center" class="fill-height">
      <v-col id="appView">
        <v-responsive class="mx-auto" max-width="600px">
        <RouterView />
        </v-responsive>
      </v-col>
    </v-row>
  </v-container>
</template>

<style>
</style>
