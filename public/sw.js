self.addEventListener("push", (event) => {
  let data = {};
  try {
    data = event.data ? event.data.json() : {};
  } catch {
    data = { title: "Trendinary alert", body: event.data ? event.data.text() : "Something changed." };
  }
  const title = data.title || "Trendinary alert";
  const options = {
    body: data.body || "A followed signal changed.",
    tag: data.tag || "trendinary-following",
    data: { url: data.url || "/following" },
    icon: "/favicon.svg",
    badge: "/favicon.svg",
  };
  event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const target = new URL(event.notification.data?.url || "/following", self.location.origin).href;
  event.waitUntil((async () => {
    const windows = await clients.matchAll({ type: "window", includeUncontrolled: true });
    for (const client of windows) {
      if (client.url.startsWith(self.location.origin) && "focus" in client) {
        await client.navigate(target);
        return client.focus();
      }
    }
    return clients.openWindow(target);
  })());
});
