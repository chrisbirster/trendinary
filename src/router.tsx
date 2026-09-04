import { createRouter } from "@solidjs/router";
import {
  AdminIndexPage,
  AdminInboxPage,
  AdminIssuePage,
  AdminIssuesPage,
  AdminNewIssuePage,
  AdminNotesPage,
  AdminQueuePage,
  AdminTrashPage,
} from "./admin";
import { AdminQualityPage } from "./admin-quality";
import { AdminSourcesPage } from "./admin-source-ops";
import { FollowingPage } from "./following";
import { HomePage, NotFoundPage } from "./pages";
import { SignalFomoPage, SignalPeepPage } from "./signal-quality-pages";
import { TrendPage } from "./trend-page";

export const Router = createRouter({
  routes: [
    { path: "/", component: HomePage },
    { path: "/peep", component: SignalPeepPage },
    { path: "/fomo", component: SignalFomoPage },
    { path: "/following", component: FollowingPage },
    { path: "/trend/:slug", component: TrendPage },
    { path: "/admin", component: AdminIndexPage },
    { path: "/admin/inbox", component: AdminInboxPage },
    { path: "/admin/queue", component: AdminQueuePage },
    { path: "/admin/notes", component: AdminNotesPage },
    { path: "/admin/issues", component: AdminIssuesPage },
    { path: "/admin/issues/new", component: AdminNewIssuePage },
    { path: "/admin/issues/:id", component: AdminIssuePage },
    { path: "/admin/sources", component: AdminSourcesPage },
    { path: "/admin/quality", component: AdminQualityPage },
    { path: "/admin/trash", component: AdminTrashPage },
    { path: "*404", component: NotFoundPage },
  ],
});

export const { paths } = Router;
