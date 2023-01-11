import { defineStore } from "pinia";
import axios from "axios";
import { login } from "masto";
import router from "../router";
import { webfinger } from "../assets/utils";

export const useMastodonStore = defineStore("mastodon", {
  state() {
    return {
      authorized: false,
      oauth: {
        url: "",
        token: "",
        audon_id: "",
      },
      client: null,
      userinfo: null,
    };
  },
  getters: {
    myWebfinger() {
      if (this.userinfo !== null) {
        return webfinger(this.userinfo);
      }
      return "";
    },
  },
  actions: {
    async fetchToken() {
      const resp = await axios.get("/api/token");
      this.oauth = resp.data;
      const client = await login({
        url: this.oauth.url,
        accessToken: this.oauth.token,
        disableVersionCheck: true,
      });
      this.client = client;
      this.userinfo = await client.accounts.verifyCredentials();
      this.authorized = true;
    },
    async callMastodonAPI(caller, ...args) {
      try {
        return await caller(...args);
      } catch (error) {
        if (error.response?.status === 401) {
          this.$reset();
          router.push({ name: "login" });
        }
        throw error;
      }
    },
  },
});
