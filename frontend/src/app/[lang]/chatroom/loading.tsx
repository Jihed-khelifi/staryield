import { RouteLoading } from "@/components/layout/route-loading";

/**
 * Chatroom skeleton while the route streams. (There is deliberately no
 * `[lang]/loading.tsx`: wrapping the catch-all page in a Suspense boundary
 * would stream unknown URLs with HTTP 200 instead of a real 404.)
 */
export default function ChatroomLoading() {
  return <RouteLoading variant="chat" />;
}
