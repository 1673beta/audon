<script>
import { mdiArrowLeft, mdiMagnify, mdiClose } from "@mdi/js";
import { debounce } from "lodash-es";
import { login } from "masto";
import { webfinger } from "../assets/utils";

export default {
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
      title: "",
      description: "",
      cohosts: [
        {
          acct: "admin",
          displayName: "Test Name",
          avatar:
            "https://mstdn.nmkj.tk/system/accounts/avatars/109/453/041/233/122/320/original/a111d61c635f57ae.png",
          url: "https://mstdn.nmkj.tk/@admin",
        },
      ],
      relationship: "everyone",
      relOptions: [
        { title: "制限なし", value: "everyone" },
        { title: "フォロー限定", value: "following" },
        { title: "フォロワー限定", value: "follower" },
        { title: "フォローまたはフォロワー限定", value: "knows" },
        { title: "相互フォロー限定", value: "mutual" },
      ],
      searchResult:
        {
          acct: "user1",
          displayName: "User 1",
          avatar:
            "https://media.songbird.cloud/accounts/avatars/109/374/604/338/855/643/original/23b2384b4bc8ccae.png",
          url: "https://mstdn.nmkj.tk/@user1",
        },
      searchQuery: "",
      isCandiadateLoading: false,
    };
  },
  methods: {
    async search(val) {
      if (!val) return;
      const webfinger = val.split("@");
      if (webfinger.length < 2) return;
      this.cohostSearch(webfinger);
      try {
        const url = new URL(`https://${webfinger[1]}`);
        this.isCandiadateLoading = true;
        const client = await login({ url: url.toString() });
        const user = await client.accounts.lookup({ acct: webfinger[0] });
        this.searchResult = [user];
      } catch {
      } finally {
        this.isCandiadateLoading = false;
      }
    },
    webfinger
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
      <v-card>
        <v-card-title class="text-center">部屋を新規作成</v-card-title>
        <v-card-text>
          <v-form>
            <v-text-field v-model="title" label="タイトル"></v-text-field>
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
            <v-card class="my-2" variant="outlined">
              <v-card-title class="text-subtitle-1">共同ホスト</v-card-title>
              <v-card-text v-if="(cohosts.length > 0 || searchResult)">
                <div v-if="(cohosts.length > 0)">
                  <v-list lines="two" variant="tonal">
                    <v-list-item
                      v-for="(cohost, index) in cohosts"
                      :key="cohost.url"
                      :title="cohost.displayName"
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
                        <v-btn variant="plain" size="small" :icon="mdiClose" @click="() => {cohosts.splice(index)}"></v-btn>
                      </template>
                    </v-list-item>
                  </v-list>
                </div>
                <div v-if="(searchResult)">
                  <v-divider></v-divider>
                  <v-list lines="two" variant="flat">
                    <v-list-item :title="searchResult.displayName">
                      <template v-slot:prepend>
                        <v-avatar class="rounded">
                          <v-img :src="searchResult.avatar"></v-img>
                        </v-avatar>
                      </template>
                      <v-list-item-subtitle>
                        {{ webfinger(searchResult) }}
                      </v-list-item-subtitle>
                      <v-list-item-action end>
                      </v-list-item-action>
                    </v-list-item>
                  </v-list>
                </div>
              </v-card-text>
              <v-card-actions>
                <v-text-field
                  density="compact"
                  v-model="searchQuery"
                  :prepend-inner-icon="mdiMagnify"
                  single-line
                  hide-details
                  clearable
                  :loading="isCandiadateLoading"
                  placeholder="user@mastodon.example"
                >
                </v-text-field>
              </v-card-actions>
            </v-card>
          </v-form>
        </v-card-text>
      </v-card>
    </div>
  </main>
</template>
