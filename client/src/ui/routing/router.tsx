import { lazy } from 'react';
import { createBrowserRouter, type RouteObject } from 'react-router';
import { RoutePaths } from './routes';
import { RootLayout } from '../components/root-layout';
import { NotFoundView } from '../views/not_found_view/not_found_view';

/**
 * Both content views are code-split: the shell (router + layout) loads first
 * and each screen's chunk is fetched on demand behind the ONE Suspense
 * boundary in RootLayout. The not-found view stays eager — the error path
 * must render even if a lazy chunk cannot load.
 */
const HomeView = lazy(() => import('../views/home_view/home_view'));
const TopicView = lazy(() => import('../views/topic_view/topic_view'));

/**
 * The route tree. Exposed as a function so the app builds a browser router and
 * tests can build an equivalent memory router from the same objects.
 */
export function appRoutes(): RouteObject[] {
  return [
    {
      element: <RootLayout />,
      children: [
        {
          path: RoutePaths.Home,
          element: <HomeView />,
        },
        {
          path: RoutePaths.Topic,
          element: <TopicView />,
        },
        {
          path: RoutePaths.NotFound,
          element: <NotFoundView />,
        },
      ],
    },
  ];
}

/**
 * The production router — created once in main.tsx. The instance type is
 * inferred (react-router's exported `Router` name is the <Router/> component,
 * not the router-instance type).
 */
export function createAppRouter() {
  return createBrowserRouter(appRoutes());
}
