import { defineStore } from "pinia";
import axios from "axios";

export const useMastodonStore = defineStore("mastodon", {
  state() {
    return {
      oauth: null,
      authorized: false
    };
  },
  actions: {
    async fetchToken() {
      try {
        const resp = await axios.get("/api/token");
        this.$state.oauth = resp.data;
        this.$state.authorized = true
      } catch (error) {
        if (error.response?.status !== 401) {
          alert(`Server is down: ${error.response.status}`);
        }
      }
    },
  },
});
