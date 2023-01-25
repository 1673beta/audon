import { helpers } from "@vuelidate/validators";
import router from "../router";

export const validators = {
  fqdn: helpers.regex(
    /(?=^.{4,253}$)(^((?!-)[a-zA-Z0-9-]{1,63}(?<!-)\.)+[a-zA-Z]{2,63}$)/
  ),
};

export function webfinger(user) {
  if (!user) return "";
  const url = new URL(user.url);
  const finger = user.acct.split("@");
  return `${finger[0]}@${url.host}`;
}

export function pushNotFound(route) {
  router.push({
    name: "notfound",
    // preserve current path and remove the first char to avoid the target URL starting with `//`
    params: { pathMatch: route.path.substring(1).split("/") },
    // preserve existing query and hash if any
    query: route.query,
    hash: route.hash,
  });
}
