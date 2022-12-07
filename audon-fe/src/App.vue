<script setup>
import { RouterView } from 'vue-router'
</script>

<template>
  <v-app class="fill-height">
    <v-system-bar :height="40">
      <v-row>
        <v-col class="text-center">
          <h2>Audon</h2>
        </v-col>
      </v-row>
    </v-system-bar>
    <v-main>
      <v-container class="fill-height">
        <v-row align="center" justify="center" class="fill-height">
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
