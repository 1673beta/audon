import { defineStore } from "pinia";
import axios from "axios";
import { createClient } from "masto";
import { webfinger } from "../assets/utils";

export const useMastodonStore = defineStore("mastodon", {
  state() {
    return {
      authorized: false,
      oauth: {
        url: "",
        token: "",
        audon: null,
      },
      client: null,
      userinfo: null,
      avatar: "",
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
      const client = createClient({
        url: this.oauth.url,
        accessToken: this.oauth.token,
        disableVersionCheck: true,
      });
      this.client = client;
      const user = await client.v1.accounts.verifyCredentials();
      this.userinfo = user;
      this.authorized = true;
    },
    async updateAvatar(img, filename) {
      if (this.client === null) return;
      const avatarBlob = await (await fetch(img)).blob();
      this.userinfo = await this.client.v1.accounts.updateCredentials({
        avatar: new File([avatarBlob], `${Date.now()}_${filename}`),
      });
    },
    async revertAvatar() {
      const token = await axios.get("/api/token");
      if (token.data.audon.avatar) {
        if (this.avatar) {
          await this.updateAvatar(this.avatar, token.data.audon.avatar);
        }
        await axios.delete("/api/room");
      }
    },
  },
});
