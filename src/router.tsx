import { createRouter } from "@solidjs/router";
import { FomoPage, FollowingPage, HomePage, NotFoundPage, PeepPage } from "./pages";
import { TrendPage } from "./trend-page";

export const Router = createRouter({
  routes: [
    { path: "/", component: HomePage },
    { path: "/peep", component: PeepPage },
    { path: "/fomo", component: FomoPage },
    { path: "/following", component: FollowingPage },
    { path: "/trend/:slug", component: TrendPage },
    { path: "*404", component: NotFoundPage },
  ],
});

export const { paths } = Router;
