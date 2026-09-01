import { createSignal, For, Show } from "solid-js";
import { useParams } from "@solidjs/router";
import * as stylex from "@stylexjs/stylex";
import {
  addIssueItem,
  createIssue,
  enrichItem,
  fetchInbox,
  fetchIssue,
  fetchIssues,
  fetchIssueSuggestions,
  fetchNotes,
  fetchQueue,
  fetchSourceRuns,
  fetchSources,
  fetchTrash,
  ingestSource,
  markOpened,
  putNote,
  removeIssueItem,
  setEditorialState,
  setSourceEnabled,
  updateIssue,
  type ContentType,
  type EditorialItem,
  type EditorialSource,
  type EditorialState,
  type InboxFilters,
  type IngestionRun,
  type NewsletterIssue,
} from "./admin-api";
import { adminStyles as styles } from "./admin.stylex";

const sx = stylex.attrs;

function errorMessage(reason: unknown) {
  return reason instanceof Error ? reason.message : "Something went wrong.";
}

function AdminShell(props: { title: string; eyebrow: string; copy?: string; children: unknown }) {
  const path = window.location.pathname;
  const links = [
    ["Inbox", "/admin/inbox"],
    ["Queue", "/admin/queue"],
    ["Notes", "/admin/notes"],
    ["Issues", "/admin/issues"],
    ["Sources", "/admin/sources"],
    ["Trash", "/admin/trash"],
  ];
  return (
    <div {...sx(styles.page)}>
      <div {...sx(styles.shell)}>
        <aside {...sx(styles.sidebar)}>
          <div {...sx(styles.brand)}>Trendinary</div>
          <div {...sx(styles.adminLabel)}>Private editorial admin</div>
          <nav {...sx(styles.nav)}>
            <For each={links}>{([label, href]) => <a href={href} {...sx(styles.navLink, path === href || (href === "/admin/issues" && path.startsWith("/admin/issues/")) ? styles.navActive : undefined)}>{label}</a>}</For>
          </nav>
          <a href="/" {...sx(styles.backLink)}>← Public Trendinary</a>
        </aside>
        <main {...sx(styles.main)}>
          <header {...sx(styles.topbar)}>
            <div>
              <div {...sx(styles.eyebrow)}>{props.eyebrow}</div>
              <h1 {...sx(styles.title)}>{props.title}</h1>
              <Show when={props.copy}><p {...sx(styles.copy)}>{props.copy}</p></Show>
            </div>
          </header>
          {props.children as never}
        </main>
      </div>
    </div>
  );
}

function relativeTime(value?: string) {
  if (!value) return "—";
  const delta = Date.now() - new Date(value).getTime();
  const minutes = Math.max(0, Math.round(delta / 60000));
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.round(minutes / 60);
  if (hours < 48) return `${hours}h`;
  return `${Math.round(hours / 24)}d`;
}

function mediumLabel(item: EditorialItem) {
  return item.content_type === "repository" ? "Repo" : item.content_type.charAt(0).toUpperCase() + item.content_type.slice(1);
}

function EditorialCard(props: {
  item: EditorialItem;
  onState?: (item: EditorialItem, state: EditorialState) => void;
  queueMode?: boolean;
  trashMode?: boolean;
}) {
  const [busy, setBusy] = createSignal(false);
  const open = async () => {
    window.open(props.item.original_url, "_blank", "noopener,noreferrer");
    void markOpened(props.item.id).catch(() => undefined);
  };
  const transition = async (state: EditorialState) => {
    setBusy(true);
    try {
      await setEditorialState(props.item.id, state);
      props.onState?.(props.item, state);
    } finally {
      setBusy(false);
    }
  };
  return (
    <article {...sx(styles.card)}>
      <div>
        <div {...sx(styles.meta)}>
          <span>{mediumLabel(props.item)}</span><span>{props.item.publisher || props.item.publisher_domain}</span><span>{props.item.source_age_text || relativeTime(props.item.discovered_at)}</span>
          <span>{props.item.discovery_source}</span>
          <Show when={props.item.serendipity}><span {...sx(styles.badge, styles.weird)}>Serendipity</span></Show>
        </div>
        <h2 {...sx(styles.headline)}>{props.item.title}</h2>
        <Show when={props.item.description}><p {...sx(styles.description)}>{props.item.description}</p></Show>
        <div {...sx(styles.why)}>Relevant because: {props.item.why_interesting}</div>
        <div {...sx(styles.actions)}>
          <button {...sx(styles.button)} onClick={open}>Open</button>
          <Show when={!props.trashMode && !props.queueMode}><button disabled={busy()} {...sx(styles.button)} onClick={() => void transition("queued")}>Queue</button></Show>
          <Show when={!props.trashMode}><button disabled={busy()} {...sx(styles.button)} onClick={() => void transition("saved")}>Save</button></Show>
          <Show when={props.queueMode}><button disabled={busy()} {...sx(styles.button, styles.buttonAccent)} onClick={() => void transition("consumed")}>Mark consumed</button></Show>
          <Show when={!props.trashMode}><button disabled={busy()} {...sx(styles.button, styles.buttonDanger)} onClick={() => void transition("rejected")}>Reject</button></Show>
          <Show when={props.trashMode}><button disabled={busy()} {...sx(styles.button, styles.buttonAccent)} onClick={() => void transition("inbox")}>Restore</button><button disabled={busy()} {...sx(styles.button)} onClick={() => void transition("archived")}>Archive</button></Show>
          <Show when={props.item.enrichment_status === "pending" || props.item.enrichment_status === "failed"}><button {...sx(styles.button, styles.buttonQuiet)} onClick={() => void enrichItem(props.item.id)}>Enrich</button></Show>
        </div>
      </div>
      <div>
        <div {...sx(styles.score)}>{props.item.editorial_score}</div>
        <div {...sx(styles.scoreLabel)}>editorial score</div>
      </div>
    </article>
  );
}

export function AdminInboxPage() {
  const [items, setItems] = createSignal<EditorialItem[]>([]);
  const [error, setError] = createSignal<string>();
  const [loading, setLoading] = createSignal(true);
  const [type, setType] = createSignal<ContentType | "">("");
  const [period, setPeriod] = createSignal<"" | "today" | "week">("");
  const [sort, setSort] = createSignal<"recommended" | "newest" | "score">("recommended");
  const [serendipity, setSerendipity] = createSignal(false);
  const load = () => {
    setLoading(true); setError(undefined);
    const filters: InboxFilters = { type: type(), period: period(), sort: sort(), serendipity: serendipity() };
    void fetchInbox(filters).then(setItems).catch((reason) => setError(errorMessage(reason))).finally(() => setLoading(false));
  };
  load();
  const removed = (item: EditorialItem) => setItems((values) => values.filter((value) => value.id !== item.id));
  return <AdminShell eyebrow="Editorial inbox" title="What deserves your attention?" copy="Ranked for editorial usefulness, not TechURLs position or raw popularity.">
    <div {...sx(styles.toolbar)}>
      <select {...sx(styles.select)} value={type()} onChange={(event) => { setType(event.currentTarget.value as ContentType | ""); load(); }}><option value="">All</option><option value="article">Articles</option><option value="video">Videos</option><option value="podcast">Podcasts</option><option value="repository">Repositories</option><option value="discussion">Discussions</option></select>
      <select {...sx(styles.select)} value={period()} onChange={(event) => { setPeriod(event.currentTarget.value as "" | "today" | "week"); load(); }}><option value="">Any time</option><option value="today">Today</option><option value="week">This week</option></select>
      <select {...sx(styles.select)} value={sort()} onChange={(event) => { setSort(event.currentTarget.value as "recommended" | "newest" | "score"); load(); }}><option value="recommended">Recommended</option><option value="newest">Newest</option><option value="score">Score</option></select>
      <button {...sx(styles.button, serendipity() ? styles.buttonAccent : undefined)} onClick={() => { setSerendipity(!serendipity()); load(); }}>Serendipity</button>
    </div>
    <Show when={error()}>{(value) => <div {...sx(styles.error)}>{value()}</div>}</Show>
    <Show when={!loading()} fallback={<div {...sx(styles.status)}>Loading editorial discoveries…</div>}>
      <Show when={items().length > 0} fallback={<div {...sx(styles.empty)}>Inbox is empty. Fetch TechURLs from Sources.</div>}><div {...sx(styles.list)}><For each={items()}>{(item) => <EditorialCard item={item} onState={removed} />}</For></div></Show>
    </Show>
  </AdminShell>;
}

export function AdminQueuePage() {
  const [items, setItems] = createSignal<EditorialItem[]>([]); const [error, setError] = createSignal<string>();
  void fetchQueue().then(setItems).catch((reason) => setError(errorMessage(reason)));
  const removed = (item: EditorialItem) => setItems((values) => values.filter((value) => value.id !== item.id));
  return <AdminShell eyebrow="Reading · watching · listening" title="Queue" copy="Things you intentionally decided to consume."><Show when={error()}>{(v) => <div {...sx(styles.error)}>{v()}</div>}</Show><Show when={items().length} fallback={<div {...sx(styles.empty)}>Nothing queued.</div>}><div {...sx(styles.list)}><For each={items()}>{(item) => <EditorialCard item={item} queueMode onState={removed} />}</For></div></Show></AdminShell>;
}

function NoteEditor(props: { item: EditorialItem }) {
  const [text, setText] = createSignal(props.item.note?.note ?? "");
  const [worth, setWorth] = createSignal<boolean | undefined>(props.item.note?.worth_sharing);
  const [saved, setSaved] = createSignal(false);
  const save = async () => { await putNote(props.item.id, text(), worth()); setSaved(true); window.setTimeout(() => setSaved(false), 1400); };
  return <div {...sx(styles.panel)}>
    <div {...sx(styles.meta)}><span>{props.item.publisher}</span><span>{relativeTime(props.item.discovered_at)}</span></div>
    <h2 {...sx(styles.headline)}>{props.item.title}</h2>
    <div {...sx(styles.actions)}><span {...sx(styles.copy)}>Worth sharing?</span><button {...sx(styles.button, worth() === true ? styles.buttonAccent : undefined)} onClick={() => setWorth(true)}>Yes</button><button {...sx(styles.button, worth() === false ? styles.buttonAccent : undefined)} onClick={() => setWorth(false)}>No</button></div>
    <textarea {...sx(styles.noteArea)} value={text()} placeholder="What stuck with you?" onInput={(event) => setText(event.currentTarget.value)} />
    <div {...sx(styles.actions)}><button {...sx(styles.button, styles.buttonAccent)} onClick={() => void save()}>Save note</button><Show when={saved()}><span {...sx(styles.success)}>Saved</span></Show></div>
  </div>;
}

export function AdminNotesPage() {
  const [items, setItems] = createSignal<EditorialItem[]>([]); const [error, setError] = createSignal<string>();
  void fetchNotes().then(setItems).catch((reason) => setError(errorMessage(reason)));
  return <AdminShell eyebrow="Your reactions" title="Notes" copy="The raw material for future newsletter issues. Your words stay distinct from generated suggestions."><Show when={error()}>{(v) => <div {...sx(styles.error)}>{v()}</div>}</Show><Show when={items().length} fallback={<div {...sx(styles.empty)}>Consume or save something, then leave a reaction.</div>}><div {...sx(styles.panelGrid)}><For each={items()}>{(item) => <NoteEditor item={item} />}</For></div></Show></AdminShell>;
}

export function AdminTrashPage() {
  const [items, setItems] = createSignal<EditorialItem[]>([]); void fetchTrash().then(setItems);
  const removed = (item: EditorialItem) => setItems((values) => values.filter((value) => value.id !== item.id));
  return <AdminShell eyebrow="Preference signal" title="Rejected" copy="Rejected material is retained so future recommendations can learn what is not worth your time."><Show when={items().length} fallback={<div {...sx(styles.empty)}>Nothing rejected.</div>}><div {...sx(styles.list)}><For each={items()}>{(item) => <EditorialCard item={item} trashMode onState={removed} />}</For></div></Show></AdminShell>;
}

export function AdminSourcesPage() {
  const [sources, setSources] = createSignal<EditorialSource[]>([]); const [runs, setRuns] = createSignal<IngestionRun[]>([]); const [busy, setBusy] = createSignal<string>(); const [error, setError] = createSignal<string>();
  const load = () => void fetchSources().then(setSources).catch((reason) => setError(errorMessage(reason))); load();
  const ingest = async (source: EditorialSource) => { setBusy(source.id); setError(undefined); try { await ingestSource(source.id); setRuns(await fetchSourceRuns(source.id)); load(); } catch (reason) { setError(errorMessage(reason)); } finally { setBusy(undefined); } };
  const toggle = async (source: EditorialSource) => { setSources((values) => values.map((value) => value.id === source.id ? { ...value, enabled: !value.enabled } : value)); try { setSources(await setSourceEnabled(source.id, !source.enabled)); } catch (reason) { setError(errorMessage(reason)); load(); } };
  return <AdminShell eyebrow="Discovery adapters" title="Sources" copy="TechURLs is the first private editorial discovery source. Fetches are manual today and scheduler-ready later.">
    <Show when={error()}>{(v) => <div {...sx(styles.error)}>{v()}</div>}</Show>
    <For each={sources()}>{(source) => <div {...sx(styles.sourceRow)}><div><strong>{source.name}</strong><div {...sx(styles.copy)}>{source.kind} · {source.url}</div></div><span>{source.enabled ? "ENABLED" : "DISABLED"}</span><span>last {relativeTime(source.last_successful_at)}</span><span>{source.latest_item_count} items</span><div {...sx(styles.actions)}><button {...sx(styles.button)} onClick={() => void toggle(source)}>{source.enabled ? "Disable" : "Enable"}</button><button disabled={!source.enabled || busy() === source.id} {...sx(styles.button, styles.buttonAccent)} onClick={() => void ingest(source)}>{busy() === source.id ? "Fetching…" : "Fetch now"}</button></div></div>}</For>
    <Show when={runs().length}><section {...sx(styles.issueSection)}><h2 {...sx(styles.sectionTitle)}>Last ingestion</h2><For each={runs().slice(0, 5)}>{(run) => <div {...sx(styles.itemLine)}><span>{run.status} · {new Date(run.started_at).toLocaleString()}</span><span>{run.metrics.items_inserted} new · {run.metrics.duplicates} dupes · {run.metrics.malformed} malformed</span></div>}</For></section></Show>
  </AdminShell>;
}

export function AdminIssuesPage() {
  const [issues, setIssues] = createSignal<NewsletterIssue[]>([]); void fetchIssues().then(setIssues);
  return <AdminShell eyebrow="Editorial output" title="Newsletter issues" copy="Assembly only. Sending is deliberately not part of this phase."><div {...sx(styles.toolbar)}><a href="/admin/issues/new" {...sx(styles.button, styles.buttonAccent)}>New issue</a></div><Show when={issues().length} fallback={<div {...sx(styles.empty)}>No drafts yet.</div>}><div {...sx(styles.list)}><For each={issues()}>{(issue) => <a href={`/admin/issues/${issue.id}`} {...sx(styles.card)} style={{ "text-decoration": "none", color: "inherit" }}><div><div {...sx(styles.meta)}>{issue.status} · {issue.issue_date || "undated"}</div><h2 {...sx(styles.headline)}>{issue.title}</h2></div><span>→</span></a>}</For></div></Show></AdminShell>;
}

export function AdminNewIssuePage() {
  const [title, setTitle] = createSignal(`Issue — ${new Date().toLocaleDateString()}`); const [issueDate, setIssueDate] = createSignal(new Date().toISOString().slice(0, 10)); const [suggestions, setSuggestions] = createSignal<EditorialItem[]>([]); const [selected, setSelected] = createSignal<Set<string>>(new Set()); const [error, setError] = createSignal<string>();
  void fetchIssueSuggestions().then(setSuggestions).catch((reason) => setError(errorMessage(reason)));
  const toggle = (id: string) => setSelected((current) => { const next = new Set(current); if (next.has(id)) next.delete(id); else next.add(id); return next; });
  const create = async () => { try { const issue = await createIssue({ title: title(), issue_date: issueDate(), status: "draft" }); let position = 0; for (const item of suggestions()) { if (selected().has(item.id)) await addIssueItem(issue.id, { content_item_id: item.id, section: position < 3 ? "thinking" : "worth_your_time", position: position++ }); } window.location.href = `/admin/issues/${issue.id}`; } catch (reason) { setError(errorMessage(reason)); } };
  return <AdminShell eyebrow="New draft" title="Build an issue" copy="Suggestions come strictly from saved/consumed items with your notes. Trendinary does not invent your opinions."><Show when={error()}>{(v) => <div {...sx(styles.error)}>{v()}</div>}</Show><div {...sx(styles.issueForm)}><input {...sx(styles.input)} value={title()} onInput={(event) => setTitle(event.currentTarget.value)} /><input type="date" {...sx(styles.input)} value={issueDate()} onInput={(event) => setIssueDate(event.currentTarget.value)} /><h2 {...sx(styles.sectionTitle)}>Available source material</h2><For each={suggestions()}>{(item) => <label {...sx(styles.itemLine)}><span><input type="checkbox" checked={selected().has(item.id)} onChange={() => toggle(item.id)} /> {item.title}</span><span>{item.note?.worth_sharing === true ? "worth sharing" : "noted"}</span></label>}</For><button {...sx(styles.button, styles.buttonAccent)} onClick={() => void create()}>Create draft</button></div></AdminShell>;
}

export function AdminIssuePage() {
  const params = useParams(); const [issue, setIssue] = createSignal<NewsletterIssue>(); const [suggestions, setSuggestions] = createSignal<EditorialItem[]>([]); const [error, setError] = createSignal<string>();
  const load = () => { if (!params.id) return; void Promise.all([fetchIssue(params.id), fetchIssueSuggestions()]).then(([value, available]) => { setIssue(value); setSuggestions(available); }).catch((reason) => setError(errorMessage(reason))); }; load();
  const save = async () => { const current = issue(); if (!current) return; try { setIssue(await updateIssue(current.id, current)); } catch (reason) { setError(errorMessage(reason)); } };
  const add = async (item: EditorialItem, section: "thinking" | "worth_your_time" | "question_source" | "misc") => { const current = issue(); if (!current) return; await addIssueItem(current.id, { content_item_id: item.id, section, position: current.items?.length ?? 0 }); load(); };
  const remove = async (id: string) => { const current = issue(); if (!current) return; await removeIssueItem(current.id, id); load(); };
  return <AdminShell eyebrow="Issue builder" title={issue()?.title ?? "Loading issue…"} copy="Source material, your reactions, and editorial structure stay visibly separate."><Show when={error()}>{(v) => <div {...sx(styles.error)}>{v()}</div>}</Show><Show when={issue()}>{(current) => <>
    <div {...sx(styles.issueForm)}><input {...sx(styles.input)} value={current().title} onInput={(e) => setIssue({ ...current(), title: e.currentTarget.value })} /><select {...sx(styles.select)} value={current().status} onChange={(e) => setIssue({ ...current(), status: e.currentTarget.value as NewsletterIssue["status"] })}><option value="draft">Draft</option><option value="ready">Ready</option><option value="published">Published</option><option value="archived">Archived</option></select><textarea {...sx(styles.noteArea)} placeholder="Intro" value={current().intro ?? ""} onInput={(e) => setIssue({ ...current(), intro: e.currentTarget.value })} /><textarea {...sx(styles.noteArea)} placeholder="One question" value={current().question ?? ""} onInput={(e) => setIssue({ ...current(), question: e.currentTarget.value })} /><button {...sx(styles.button, styles.buttonAccent)} onClick={() => void save()}>Save issue</button></div>
    <For each={["thinking", "worth_your_time", "question_source", "misc"] as const}>{(section) => <section {...sx(styles.issueSection)}><h2 {...sx(styles.sectionTitle)}>{section.replaceAll("_", " ")}</h2><For each={(current().items ?? []).filter((item) => item.section === section)}>{(item) => <div {...sx(styles.panel)}><strong>{item.content?.title}</strong><Show when={item.content?.note}><div {...sx(styles.noteText)}>{item.content?.note?.note}</div></Show><div {...sx(styles.actions)}><button {...sx(styles.button, styles.buttonDanger)} onClick={() => void remove(item.id)}>Remove</button></div></div>}</For></section>}</For>
    <section {...sx(styles.issueSection)}><h2 {...sx(styles.sectionTitle)}>Add saved/noted material</h2><For each={suggestions().filter((item) => !(current().items ?? []).some((existing) => existing.content_item_id === item.id)).slice(0, 12)}>{(item) => <div {...sx(styles.itemLine)}><span>{item.title}</span><div {...sx(styles.actions)}><button {...sx(styles.button)} onClick={() => void add(item, "thinking")}>Thinking</button><button {...sx(styles.button)} onClick={() => void add(item, "worth_your_time")}>Worth your time</button><button {...sx(styles.button)} onClick={() => void add(item, "question_source")}>Question source</button></div></div>}</For></section>
  </>}</Show></AdminShell>;
}

export function AdminIndexPage() { return <AdminInboxPage />; }
