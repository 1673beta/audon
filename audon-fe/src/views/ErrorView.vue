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
      const target = new URL(decodeURI(this.$route.query.url));
      alert(this.$t("errors.offline"));
      window.location.href = target.toString();
    }
  },
};
</script>

<template>
  <v-alert v-if="this.name === 'notfound'" type="error">Page not found</v-alert>
</template>
