import { createApp } from "vue";
import { createPinia } from "pinia";

import { createVuetify } from "vuetify";
import { aliases, mdi } from "vuetify/iconsets/mdi-svg";

import axios from "axios";

import App from "./App.vue";
import router from "./router";
import { useMastodonStore } from "./stores/mastodon";

import "./assets/style.css";
import "./assets/koruri/koruri.css";
import "vuetify/styles";

const vuetify = createVuetify({
  theme: {
    defaultTheme: "dark",
  },
  icons: {
    defaultSet: "mdi",
    aliases,
    sets: {
      mdi,
    },
  },
});

axios.defaults.withCredentials = true;

// if audon server returns 401, display the login form
axios.interceptors.response.use(undefined, (error) => {
  if (error.response?.status === 401) {
    const donStore = useMastodonStore();
    donStore.$reset();
  }
  return Promise.reject(error);
});
router.beforeEach(async (to) => {
  const donStore = useMastodonStore();
  if ((!to.meta.noauth || to.name === "login") && !donStore.authorized) {
    try {
      if (!donStore.client) await donStore.fetchToken();
    } catch (error) {
      if (error.response?.status === 401) {
        donStore.$reset();
      }
    }
  }
});
router.afterEach((to, from) => {
  const donStore = useMastodonStore();
  if (!to.meta.noauth && !donStore.authorized) {
    const query = to.name !== "home" ? {l: to.path} : {};
    router.push({ name: "login", query }); // need to push in afterEach to get nonempty lastPath in LoginView.vue
  } else if (to.name === "login" && donStore.authorized) {
    router.replace({ name: "home" });
  }
});

const app = createApp(App);

app.use(createPinia());
app.use(vuetify);
app.use(router);

// app.config.compilerOptions.delimiters = ["{%", "%}"];
app.mount("#app");
