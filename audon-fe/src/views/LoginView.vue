<script>
import { RouterLink } from "vue-router";
import { useVuelidate } from "@vuelidate/core";
import { required, helpers } from "@vuelidate/validators";
import utils from "../assets/utils";
import _ from "lodash/collection";
import axios from "axios";

export default {
  setup() {
    return {
      v$: useVuelidate(),
    };
  },
  data() {
    return {
      server: "",
      serverErr: "",
      lastPath: null,
    };
  },
  mounted() {
    const from = this.$router.options.history.state.back;
    this.lastPath = from === this.$route.path ? "/" : from;
  },
  validations() {
    return {
      server: {
        required: helpers.withMessage("アドレスを入力してください", required),
        hostname: helpers.withMessage(
          "有効なアドレスを入力してください",
          utils.validators.fqdn
        ),
      },
    };
  },
  computed: {
    serverErrors() {
      const errors = this.v$.server.$errors;
      const messages = _.map(errors, (e) => e.$message);
      if (this.serverErr !== "") {
        messages.push(this.serverErr);
      }
      return messages;
    },
  },
  methods: {
    async onSubmit() {
      const isFormCorrect = await this.v$.$validate();
      if (!isFormCorrect) {
        return;
      }
      try {
        const response = await axios.postForm("/app/login", {
          redir: this.lastPath,
          server: this.server,
        });
        if (response.status === 201) {
          // this.$router.push(response.data)
          location.assign(response.data);
          this.serverErr = "";
        }
      } catch (error) {
        if (error.response?.status === 404) {
          this.serverErr = "サーバーが見つかりません";
        }
      }
    },
    onInput() {
      this.v$.server.$touch();
      this.serverErr = "";
    },
  },
};
</script>

<template>
  <v-alert v-if="$route.query.warn" type="warning" variant="text">
    <div>ログインが必要です</div>
  </v-alert>
  <h1>Audon</h1>
  <v-form ref="form" @submit.prevent="onSubmit" class="my-3" lazy-validation>
    <v-text-field
      v-model="server"
      name="server"
      label="Mastodon / Pleroma サーバー"
      placeholder="mastodon.example"
      class="mb-2"
      :error-messages="serverErrors"
      @update:model-value="onInput"
      clearable
    />
    <v-btn block @click="onSubmit" :disabled="!v$.$dirty || v$.$error"
      >ログイン</v-btn
    >
  </v-form>
  <div class="w-100 text-right">
    <RouterLink to="/about">利用規約</RouterLink>
  </div>
</template>
