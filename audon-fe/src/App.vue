<script setup>
import { RouterView, RouterLink } from 'vue-router'
</script>

<template>
  <v-app class="fill-height">
    <v-system-bar window>
      <v-row>
        <v-col class="text-center">
          <h2><RouterLink :to="{name: 'home'}" style="text-decoration: inherit; color: inherit;">Audon</RouterLink></h2>
        </v-col>
      </v-row>
      <div style="position:fixed">v0.1.0-dev</div>
    </v-system-bar>
    <v-main>
      <v-container class="fill-height">
        <v-row align="center" justify="center" class="fill-height" id="mainArea">
          <v-col>
            <v-responsive class="mx-auto" max-width="600px">
                <RouterView />
            </v-responsive>
          </v-col>
        </v-row>
      </v-container>
    </v-main>
  </v-app>
</template>

<style>
#app .v-application__wrap {
  min-height: 100%;
}
</style>
