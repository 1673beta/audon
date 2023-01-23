<script>
import { RouterView } from "vue-router";
import locales from "./locales";

export default {
  components: {
    RouterView,
  },
  setup() {
    return {
      locales,
    };
  },
  data() {
    const appElem = document.getElementById("app");
    return {
      version: appElem.getAttribute("data-version"),
    };
  },
  methods: {
    onLocaleChange() {
      localStorage.setItem("locale", this.$i18n.locale);
    },
  },
};
</script>

<template>
  <v-app style="min-height: 100%">
    <div id="mascot" class="pa-3">
      <img :draggable="false" src="./assets/img/mascot.webp" />
    </div>
    <v-system-bar window>
      <div class="d-flex justify-center align-center w-100">
        <img
          height="20"
          src="./assets/img/audon-logo-orange.svg"
          alt="Branding Logo"
        />
      </div>
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
        <div>
          <a
            href="https://codeberg.org/nmkj/audon"
            class="plain"
            target="_blank"
            >{{ version }}</a
          >
        </div>
        <div>
          <select
            v-model="$i18n.locale"
            id="localeSelector"
            @change="onLocaleChange"
          >
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
#mascot {
  position: fixed;
  bottom: 20px;
  left: 0;
}

#mascot img {
  opacity: 1;
  width: 200px;
  max-width: 100%;
}

#app .v-application__wrap {
  min-height: 100%;
}

#localeSelector option {
  background: black;
  color: white;
}
</style>
