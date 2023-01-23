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
    async updateAvatar(img) {
      if (this.client === null) return;
      const avatarBlob = await (await fetch(img)).blob();
      this.userinfo = await this.client.v1.accounts.updateCredentials({
        avatar: new File([avatarBlob], `${Date.now()}.gif`),
      });
    },
    async revertAvatar() {
      const t = setTimeout(async () => {
        const token = await axios.get("/api/token");
        const oldAvatar = sessionStorage.getItem("avatar_old_data");
        sessionStorage.removeItem("avatar_old_data");
        sessionStorage.removeItem("avatar_timeout");
        if (this.client === null || !oldAvatar || !token.data.audon.avatar)
          return;
        const resp = await axios.delete("/api/room");
        if (resp.status === 200) {
          const avatarBlob = await (await fetch(oldAvatar)).blob();
          this.userinfo = await this.client.v1.accounts.updateCredentials({
            avatar: new File([avatarBlob], token.data.audon.avatar),
          });
        }
      }, 2 * 1000);
      sessionStorage.setItem("avatar_timeout", t.toString());
    },
  },
});
