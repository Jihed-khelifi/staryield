/** "(3) Chatroom — Staryield": the unread count in the tab title while the tab is hidden. */
const PREFIX = /^\((?:\d+|99\+)\) /;

export function stripUnreadPrefix(title: string): string {
  return title.replace(PREFIX, "");
}

export function withUnreadPrefix(title: string, count: number): string {
  const base = stripUnreadPrefix(title);
  if (count <= 0) return base;
  return `(${count > 99 ? "99+" : count}) ${base}`;
}
