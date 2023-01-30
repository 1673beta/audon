<script>
export default {
  data() {
    return {
      name: this.$route.name,
    };
  },
  created() {
    if (this.name === "offline") {
      alert(this.$t("errors.offline"));
      this.$router.push({ name: "home" });
    }
  },
};
</script>

<template>
  <v-alert v-if="this.name === 'notfound'" type="error">Page not found</v-alert>
</template>
