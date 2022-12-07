<script>
import { mdiArrowLeft, mdiMagnify, mdiClose, mdiPlus } from "@mdi/js";
import { useVuelidate } from "@vuelidate/core";
import { useMastodonStore } from "../stores/mastodon"
import { helpers, required } from "@vuelidate/validators";
import { debounce, some, map } from "lodash-es";
import { login } from "masto";
import { webfinger } from "../assets/utils";
import axios from "axios";

export default {
  setup() {
    return {
      v$: useVuelidate(),
      donStore: useMastodonStore()
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
        { title: "フォロー限定", value: "following" },
        { title: "フォロワー限定", value: "follower" },
        { title: "フォローまたはフォロワー限定", value: "knows" },
        { title: "相互フォロー限定", value: "mutual" },
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
    webfinger,
    async onSubmit() {
      const isFormCorrect = await this.v$.$validate();
      if (!isFormCorrect) {
        return;
      }
      const payload = {
        title: this.title,
        description: this.description,
        cohosts: map(this.cohosts, (u) => ({
          remote_id: u.acct,
          remote_url: u.url,
        })),
      };
      try {
        const resp = await axios.post("/api/room", payload);
        if (resp.status === 201) {
          // TODO: redirect to the created room
        }
      } catch (error) {
        this.searchError.message = `Error: ${error}`
        this.searchError.colour = "error"
        this.searchError.enabled = true
      }
    },
  },
};
</script>

<template>
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
              disabled
              :messages="['今後のアップデートで追加予定']"
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
              <v-card-text v-if="cohosts.length > 0 || searchResult">
                <div v-if="cohosts.length > 0">
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
                </div>
                <div v-if="searchResult">
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
                </div>
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
