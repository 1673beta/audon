import { createRouter, createWebHistory } from "vue-router";
import LoginView from "../views/LoginView.vue";
import RoomView from "../views/RoomView.vue";
import ErrorView from "../views/ErrorView.vue";
import axios from "axios";

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: "/:pathMatch(.*)*",
      name: "notfound",
      meta: {
        noauth: true,
      },
      component: ErrorView,
    },
    {
      path: "/error/:type",
      name: "error",
      meta: {
        noauth: true,
      },
      component: ErrorView,
    },
    {
      path: "/",
      name: "home",
      component: () => import("../views/HomeView.vue"),
    },
    {
      path: "/login",
      name: "login",
      meta: {
        noauth: true,
      },
      component: LoginView,
    },
    {
      path: "/create",
      name: "create",
      component: () => import("../views/CreateView.vue"),
    },
    {
      path: "/r/:id",
      name: "room",
      meta: {
        noauth: true,
      },
      component: RoomView,
    },
    {
      path: "/u/:webfinger",
      name: "currentHosting",
      meta: {
        noauth: true,
      },
      redirect: async (to) => {
        try {
          const resp = await axios.get(`/u/${to.params.webfinger}`);
          if (resp.status === 302) {
            return {
              name: "room",
              params: { id: resp.headers.location },
            };
          }
        } catch (error) {
          console.log(error);
        }
        return { name: "notfound" };
      },
    },
  ],
});

export default router;
