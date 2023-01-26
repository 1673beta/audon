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
import { helpers, maxLength, required } from "@vuelidate/validators";
import { debounce, some, map, truncate, trim } from "lodash-es";
import { webfinger } from "../assets/utils";
import axios from "axios";

export default {
  setup() {
    return {
      mdiMastodon,
      mdiClipboardCheck,
      mdiClipboardEdit,
      mdiArrowLeft,
      mdiMagnify,
      mdiClose,
      mdiPlus,
      webfinger,
      v$: useVuelidate(),
      donStore: useMastodonStore(),
      clipboard: useClipboard(),
    };
  },
  async created() {
    const resp = await axios.get("/api/room");
    if (resp.data.length > 0) {
      const canCreate = !some(resp.data, { role: "host" });
      if (!canCreate) {
        alert(this.$t("errors.alreadyAdded"));
        this.$router.replace({ name: "home" });
      }
    }
    this.cohostSearch = debounce(this.search, 1000);
  },
  unmounted() {
    this.cohostSearch.cancel();
  },
  data() {
    return {
      title: "",
      description: "",
      cohosts: [],
      relationship: "everyone",
      relOptions: [
        { title: this.$t("form.relationships.everyone"), value: "everyone" },
        { title: this.$t("form.relationships.following"), value: "following" },
        { title: this.$t("form.relationships.follower"), value: "follower" },
        { title: this.$t("form.relationships.knowing"), value: "knowing" },
        { title: this.$t("form.relationships.mutual"), value: "mutual" },
        { title: this.$t("form.relationships.private"), value: "private" },
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
      advertise: true,
    };
  },
  validations() {
    return {
      title: {
        required: helpers.withMessage(this.$t("form.titleRequired"), required),
        maxLength: maxLength(100),
      },
      description: {
        maxLength: maxLength(500),
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
        this.$t("shareRoomMessage", {
          link: this.donStore.myStaticLink,
          title: this.title,
        }),
      ];
      if (this.description)
        texts.push(truncate("\n" + this.description, { length: 200 }));
      return encodeURI(`${url.origin}/share?text=${texts.join("\n")}`);
    },
  },
  watch: {
    searchQuery(val) {
      this.isCandiadateLoading = false;
      this.cohostSearch.cancel();
      if (!val) return;
      if (some(this.cohosts, { finger: val })) {
        this.searchError.message = this.$t("errors.alreadyAdded");
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
    relationship(to) {
      this.advertise = to === "everyone";
    },
  },
  methods: {
    async search(val) {
      const finger = val.split("@");
      if (finger.length !== 2) return;
      try {
        const resp = await this.donStore.client.v1.accounts.search({
          q: val,
          resolve: true,
        });
        if (resp.length != 1) throw "";
        const user = resp[0];
        user.finger = webfinger(user);
        this.searchResult = user;
        this.searchError.enabled = false;
      } catch (error) {
        this.searchError.message = this.$t("errors.notFound", { value: val });
        this.searchError.colour = "error";
        this.searchError.enabled = true;
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
        restriction: this.relationship,
        advertise:
          this.advertise && this.relationship === "everyone"
            ? this.$i18n.locale
            : "",
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
    <v-alert type="success" color="blue-gray" :title="$t('roomReady.header')">
      <div>
        {{ $t("roomReady.message", { title }) }}
      </div>
      <div class="my-3">
        <h3 style="word-break: break-all">{{ donStore.myStaticLink }}</h3>
      </div>
      <div>
        <v-btn
          :prepend-icon="mdiMastodon"
          class="mr-3"
          @click="onShareClick"
          color="#563ACC"
          size="small"
          >{{ $t("share") }}</v-btn
        >
        <v-btn
          @click="clipboard.copy(donStore.myStaticLink)"
          color="lime"
          size="small"
          :prepend-icon="
            clipboard.copied.value ? mdiClipboardCheck : mdiClipboardEdit
          "
          >{{ clipboard.copied.value ? $t("copied") : $t("copy") }}</v-btn
        >
      </div>
      <v-alert class="mt-5" density="compact" type="warning" variant="tonal">{{
        $t("roomReady.timeout", { minutes: 5 })
      }}</v-alert>
      <div class="text-center mt-5 mb-1">
        <v-btn
          color="indigo"
          :to="{ name: 'room', params: { id: createdRoomID } }"
          size="large"
          >{{ $t("enterRoom") }}</v-btn
        >
      </div>
    </v-alert>
  </v-dialog>
  <main>
    <div>
      <v-btn class="ma-2" variant="text" color="blue" :to="{ name: 'home' }">
        <v-icon start :icon="mdiArrowLeft"></v-icon>
        {{ $t("back") }}
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
        <v-card-title class="text-center">{{
          $t("createNewRoom")
        }}</v-card-title>
        <v-card-text>
          <v-form>
            <v-text-field
              v-model="title"
              :label="$t('form.title')"
              :counter="100"
              :error-messages="titleErrors"
              required
              @input="v$.title.$touch()"
              @blur="v$.title.$touch()"
            ></v-text-field>
            <v-textarea
              auto-grow
              v-model="description"
              rows="2"
              :label="$t('form.description')"
              :counter="500"
            ></v-textarea>
            <v-select
              :items="relOptions"
              :label="$t('form.restriction')"
              v-model="relationship"
              :messages="[$t('form.cohostCanAlwaysJoin')]"
            ></v-select>
            <v-card class="my-3" variant="outlined">
              <v-card-title class="text-subtitle-1">{{
                $t("form.cohosts")
              }}</v-card-title>
              <v-card-text
                v-if="cohosts.length > 0 || searchResult"
                class="py-0"
              >
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
            <v-text-field
              type="datetime-local"
              v-model="scheduledAt"
              :label="$t('form.schedule')"
              disabled
              :messages="[$t('comingFuture')]"
            ></v-text-field>
            <v-checkbox
              v-model="advertise"
              :disabled="relationship !== 'everyone'"
              density="compact"
            >
              <template v-slot:label>
                <i18n-t keypath="form.advertise" tag="div">
                  <template v-slot:bot>
                    <a
                      href="https://akkoma.audon.space/users/now"
                      target="_blank"
                      >now@audon.space</a
                    >
                  </template>
                </i18n-t>
              </template>
            </v-checkbox>
          </v-form>
        </v-card-text>
        <v-card-actions>
          <v-btn
            block
            :disabled="isSubmissionLoading"
            color="indigo"
            @click="onSubmit"
            variant="flat"
          >
            {{ $t("create") }}
          </v-btn>
        </v-card-actions>
      </v-card>
    </div>
  </main>
</template>
