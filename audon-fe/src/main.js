import { createApp } from "vue";
import { createPinia } from "pinia";

import { createVuetify } from "vuetify";
import { aliases, mdi } from "vuetify/iconsets/mdi-svg";

import App from "./App.vue";
import router from "./router";

import "./assets/style.css";
import "vuetify/styles";

const vuetify = createVuetify({
  theme: {
    defaultTheme: "dark",
  },
  icons: {
    aliases,
    sets: {
      mdi,
    },
  },
});

const app = createApp(App);

app.use(createPinia());
app.use(vuetify);
app.use(router);

app.config.compilerOptions.delimiters = ["{%", "%}"];
app.mount("#app");
