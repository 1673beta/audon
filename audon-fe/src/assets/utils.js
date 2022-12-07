import { helpers } from "@vuelidate/validators";

export const validators = {
  fqdn: helpers.regex(
    /^[a-zA-Z]([a-zA-Z0-9\-]+[\.]?)*[a-zA-Z0-9]$/,
    /^([a-zA-Z0-9]{1}[a-zA-Z0-9-]{0,62})(\.[a-zA-Z0-9]{1}[a-zA-Z0-9-]{0,62})*?(\.[a-zA-Z]{1}[a-zA-Z0-9]{0,62})\.?$/
  ),
}

export function webfinger(user) {
  const url = new URL(user.url)
  return `${user.acct}@${url.host}`
}
