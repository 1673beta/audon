<script>
import { RouterLink } from "vue-router";
import { useVuelidate } from "@vuelidate/core";
import { required, helpers } from "@vuelidate/validators";
import { validators } from "../assets/utils";
import { map } from "lodash-es";
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
    };
  },
  validations() {
    return {
      server: {
        required: helpers.withMessage(this.$t("addressRequired"), required),
        hostname: helpers.withMessage(
          this.$t("invalidAddress"),
          validators.fqdn
        ),
      },
    };
  },
  computed: {
    serverErrors() {
      const errors = this.v$.server.$errors;
      const messages = map(errors, (e) => e.$message);
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
          redir: this.$route.query.l ?? "/",
          server: this.server,
        });
        if (response.status === 201) {
          this.serverErr = "";
          location.assign(response.data);
        }
      } catch (error) {
        if (error.response?.status === 404) {
          this.serverErr = this.$t("serverNotFound");
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
  <v-alert v-if="$route.query.l" type="warning" variant="text">
    <div>{{ $t("loginRequired") }}</div>
  </v-alert>
  <v-form ref="form" @submit.prevent="onSubmit" class="my-3" lazy-validation>
    <v-text-field
      v-model="server"
      name="server"
      :label="$t('server')"
      placeholder="mastodon.example"
      class="mb-2"
      :error-messages="serverErrors"
      @update:model-value="onInput"
      type="url"
      clearable
    />
    <v-btn block @click="onSubmit" :disabled="!v$.$dirty || v$.$error"
      >{{ $t("login") }}</v-btn
    >
  </v-form>
  <div class="w-100 text-right">
    <RouterLink to="/about">{{ $t("about") }}</RouterLink>
  </div>
</template>
