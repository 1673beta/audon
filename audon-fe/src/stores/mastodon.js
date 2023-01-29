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
    myStaticLink() {
      if (this.oauth.audon?.webfinger) {
        const url = new URL(location.href);
        return `${url.origin}/u/@${this.oauth.audon.webfinger}`;
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
      const rooms = await axios.get("/api/room");
      if (
        token.data.audon.avatar &&
        (rooms.data.length === 0 ||
          (rooms.data.length === 1 && rooms.data[0].role === "host"))
      ) {
        if (this.avatar) {
          await this.updateAvatar(this.avatar, token.data.audon.avatar);
        }
        await axios.delete("/api/room");
      }
    },
  },
});
