<script>
import {
  mdiArrowLeft,
  mdiMagnify,
  mdiClose,
  mdiPlus,
  mdiClipboardCheck,
  mdiClipboardEdit,
  mdiMastodon,
} from "@mdi/js";
import { useVuelidate } from "@vuelidate/core";
import { useClipboard } from "@vueuse/core";
import { useMastodonStore } from "../stores/mastodon";
import { helpers, required } from "@vuelidate/validators";
import { debounce, some, map, truncate, trim } from "lodash-es";
import { login } from "masto";
import { webfinger } from "../assets/utils";
import axios from "axios";

export default {
  setup() {
    return {
      v$: useVuelidate(),
      donStore: useMastodonStore(),
      clipboard: useClipboard(),
    };
  },
  created() {
    this.cohostSearch = debounce(this.search, 1000);
  },
  unmounted() {
    this.cohostSearch.cancel();
  },
  data() {
    return {
      mdiMastodon,
      mdiClipboardCheck,
      mdiClipboardEdit,
      mdiArrowLeft,
      mdiMagnify,
      mdiClose,
      mdiPlus,
      title: "",
      description: "",
      cohosts: [],
      relationship: "everyone",
      relOptions: [
        { title: "制限なし", value: "everyone" },
        { title: "あなたのフォロー限定", value: "following" },
        { title: "あなたのフォロワー限定", value: "follower" },
        { title: "あなたのフォローまたはフォロワー限定", value: "knowing" },
        { title: "あなたの相互フォロー限定", value: "mutual" },
        { title: "共同ホスト限定", value: "private" },
      ],
      scheduledAt: null,
      searchResult: null,
      searchQuery: "",
      isCandiadateLoading: false,
      searchError: {
        enabled: false,
        message: "",
        timeout: 5000,
        colour: "",
      },
      isSubmissionLoading: false,
      createdRoomID: "",
    };
  },
  validations() {
    return {
      title: {
        required: helpers.withMessage("部屋の名前を入力してください", required),
      },
    };
  },
  computed: {
    titleErrors() {
      const errors = this.v$.title.$errors;
      const messages = map(errors, (e) => e.$message);
      return messages;
    },
    isDialogActive() {
      return this.createdRoomID !== "";
    },
    roomURL() {
      const url = new URL(window.location.href);
      return `${url.origin}/r/${this.createdRoomID}`;
    },
    shareURL() {
      const donURL = this.donStore.userinfo?.url;
      if (!donURL) return "";
      const url = new URL(donURL);
      const texts = [
        `Audon で部屋を作りました！\n参加用リンク: ${this.roomURL}`,
        `タイトル：${this.title}`,
      ];
      if (this.description)
        texts.push(truncate(this.description, { length: 200 }));
      return encodeURI(`${url.origin}/share?text=${texts.join("\n\n")}`);
    },
  },
  watch: {
    searchQuery(val) {
      this.isCandiadateLoading = false;
      this.cohostSearch.cancel();
      if (!val) return;
      if (some(this.cohosts, { finger: val })) {
        this.searchError.message = "すでに追加済みです";
        this.searchError.colour = "warning";
        this.searchError.enabled = true;
        return;
      }
      if (val === this.donStore.myWebfinger) {
        return;
      }
      this.isCandiadateLoading = true;
      this.cohostSearch(val);
    },
  },
  methods: {
    async search(val) {
      const finger = val.split("@");
      if (finger.length < 2) return;
      else if (finger.length === 3) {
        finger.splice(0, 1);
        this.searchQuery = finger.join("@");
      }
      try {
        const url = new URL(`https://${finger[1]}`);
        const client = await login({
          url: url.toString(),
          disableVersionCheck: true,
        });
        const user = await client.accounts.lookup({ acct: finger[0] });
        user.finger = webfinger(user);
        this.searchResult = user;
      } catch (error) {
        if (error.isMastoError && error.statusCode === 404) {
          this.searchError.message = `${val} が見つかりません`;
          this.searchError.colour = "error";
          this.searchError.enabled = true;
        }
      } finally {
        this.isCandiadateLoading = false;
      }
    },
    onResultClick() {
      this.cohosts.push(this.searchResult);
      this.searchResult = null;
      this.searchQuery = "";
    },
    onShareClick() {
      window.open(this.shareURL, "Audon Share", "width=400,height=600");
    },
    webfinger,
    async onSubmit() {
      this.title = trim(this.title);
      this.description = trim(this.description);
      const isFormCorrect = await this.v$.$validate();
      if (!isFormCorrect) {
        return;
      }
      const payload = {
        title: this.title,
        description: this.description,
        cohosts: map(this.cohosts, (u) => ({
          remote_id: u.id,
          remote_url: u.url,
        })),
        restriction: this.relationship
      };
      this.isSubmissionLoading = false;
      try {
        const resp = await axios.post("/api/room", payload);
        if (resp.status === 201) {
          this.createdRoomID = resp.data;
          // this.$router.push({ name: "room", params: { id: resp.data } });
        }
      } catch (error) {
        this.searchError.message = `Error: ${error}`;
        this.searchError.colour = "error";
        this.searchError.enabled = true;
      } finally {
        this.isSubmissionLoading = false;
      }
    },
  },
};
</script>

<template>
  <v-dialog v-model="isDialogActive" persistent max-width="700">
    <v-alert
      type="success"
      color="blue-gray"
      title="お部屋の用意ができました！"
    >
      <div>
        {{ title }} を作りました。参加者に以下の URL を共有してください。
      </div>
      <div class="my-3">
        <h3 style="word-break: break-all;">{{ roomURL }}</h3>
      </div>
      <div>
        <v-btn
          :prepend-icon="mdiMastodon"
          class="mr-3"
          @click="onShareClick"
          color="#563ACC"
          size="small"
          >シェア</v-btn
        >
        <v-btn
          @click="clipboard.copy(roomURL)"
          color="lime"
          size="small"
          :prepend-icon="
            clipboard.copied.value ? mdiClipboardCheck : mdiClipboardEdit
          "
          >{{ clipboard.copied.value ? "コピーしました" : "コピー" }}</v-btn
        >
      </div>
      <div class="text-center mt-10 mb-1">
        <v-btn
          color="indigo"
          :to="{ name: 'room', params: { id: createdRoomID } }"
          size="large"
          >入室</v-btn
        >
      </div>
    </v-alert>
  </v-dialog>
  <main>
    <div>
      <v-btn class="ma-2" variant="text" color="blue" :to="{ name: 'home' }">
        <v-icon start :icon="mdiArrowLeft"></v-icon>
        戻る
      </v-btn>
      <v-snackbar
        v-model="searchError.enabled"
        color="error"
        :timeout="searchError.timeout"
        position="sticky"
        location="top"
      >
        {{ searchError.message }}
      </v-snackbar>
      <v-card :loading="isSubmissionLoading">
        <v-card-title class="text-center">部屋を新規作成</v-card-title>
        <v-card-text>
          <v-form>
            <v-text-field
              v-model="title"
              label="タイトル"
              :error-messages="titleErrors"
              required
              @input="v$.title.$touch()"
              @blur="v$.title.$touch()"
            ></v-text-field>
            <v-textarea
              auto-grow
              v-model="description"
              rows="2"
              label="説明"
            ></v-textarea>
            <v-select
              :items="relOptions"
              label="入室制限"
              v-model="relationship"
              :messages="['共同ホストは制限に関わらず入室できます']"
            ></v-select>
            <v-text-field
              type="datetime-local"
              v-model="scheduledAt"
              label="開始予約"
              disabled
              :messages="['今後のアップデートで追加予定']"
            ></v-text-field>
            <v-card class="mt-3" variant="outlined">
              <v-card-title class="text-subtitle-1">共同ホスト</v-card-title>
              <v-card-text v-if="cohosts.length > 0 || searchResult" class="py-0">
                <template v-if="cohosts.length > 0">
                  <v-list lines="two" variant="tonal">
                    <v-list-item
                      v-for="(cohost, index) in cohosts"
                      :key="cohost.url"
                      :title="cohost.displayName"
                      class="my-1"
                      rounded
                    >
                      <template v-slot:prepend>
                        <v-avatar class="rounded">
                          <v-img :src="cohost.avatar"></v-img>
                        </v-avatar>
                      </template>
                      <template v-slot:subtitle>
                        {{ webfinger(cohost) }}
                      </template>
                      <template v-slot:append>
                        <v-btn
                          variant="text"
                          size="small"
                          :icon="mdiClose"
                          @click="
                            () => {
                              cohosts.splice(index, 1);
                            }
                          "
                        ></v-btn>
                      </template>
                    </v-list-item>
                  </v-list>
                </template>
                <template v-if="searchResult">
                  <v-divider></v-divider>
                  <v-list lines="two">
                    <v-list-item
                      :key="0"
                      :value="searchResult.acct"
                      :title="searchResult.displayName"
                      @click="onResultClick"
                    >
                      <template v-slot:prepend>
                        <v-avatar class="rounded">
                          <v-img :src="searchResult.avatar"></v-img>
                        </v-avatar>
                      </template>
                      <template v-slot:append>
                        <v-btn
                          size="small"
                          variant="plain"
                          :icon="mdiPlus"
                        ></v-btn>
                      </template>
                      <v-list-item-subtitle>
                        {{ webfinger(searchResult) }}
                      </v-list-item-subtitle>
                    </v-list-item>
                  </v-list>
                </template>
              </v-card-text>
              <v-card-actions>
                <v-text-field
                  density="compact"
                  v-model="searchQuery"
                  type="email"
                  :prepend-inner-icon="mdiMagnify"
                  single-line
                  hide-details
                  clearable
                  :error="searchError.enabled"
                  :loading="isCandiadateLoading"
                  placeholder="user@mastodon.example"
                >
                </v-text-field>
              </v-card-actions>
            </v-card>
          </v-form>
        </v-card-text>
        <v-card-actions>
          <v-btn block color="indigo" @click="onSubmit" variant="flat">
            作成
          </v-btn>
        </v-card-actions>
      </v-card>
    </div>
  </main>
</template>
