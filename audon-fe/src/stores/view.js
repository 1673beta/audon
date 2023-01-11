import { defineStore } from "pinia";

export const useViewStore = defineStore("view", {
  state() {
    return {
      mainArea: null,
    };
  },
});
