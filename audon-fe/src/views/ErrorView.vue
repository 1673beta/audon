<script>
export default {
  data() {
    return {
      name: this.$route.name,
    };
  },
  created() {
    const type = this.$route.params.type;
    if (!type) return;

    if (type === "offline") {
      alert(this.$t("errors.offline"));
      this.$router.push({ name: "home" });
    }
  },
};
</script>

<template>
  <v-alert v-if="this.name === 'notfound'" type="error">Page not found</v-alert>
</template>
