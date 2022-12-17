<script>
import { RouterView, RouterLink } from "vue-router";
import locales from "./locales"

export default {
  setup() {
    return {
      locales
    }
  },
  methods: {
    onLocaleChange() {
      localStorage.setItem("locale", this.$i18n.locale);
    }
  }
}
</script>

<template>
  <v-app class="fill-height">
    <v-system-bar window>
      <h2 class="text-center w-100">
        <RouterLink
          :to="{ name: 'home' }"
          style="text-decoration: inherit; color: inherit;"
          >Audon</RouterLink
        >
      </h2>
    </v-system-bar>
    <v-main>
      <v-container class="fill-height">
        <v-row
          align="center"
          justify="center"
          class="fill-height"
          id="mainArea"
        >
          <v-col>
            <v-responsive class="mx-auto" max-width="600px">
              <RouterView />
            </v-responsive>
          </v-col>
        </v-row>
      </v-container>
    </v-main>
    <v-bottom-navigation :height="30">
      <div class="w-100 d-flex justify-space-between align-center px-3">
        <div>v0.1.0-dev4</div>
        <div>
          <select v-model="$i18n.locale" id="localeSelector" @change="onLocaleChange">
            <option
              v-for="locale in $i18n.availableLocales"
              :key="`locale-${locale}`"
              :value="locale"
            >
              {{ locales[locale] }}
            </option>
          </select>
        </div>
      </div>
    </v-bottom-navigation>
  </v-app>
</template>

<style>
#app .v-application__wrap {
  min-height: 100%;
}

#localeSelector option {
  background: black;
  color: white;
}
</style>
