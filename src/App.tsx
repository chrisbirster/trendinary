import type { ParentProps } from "solid-js";
import * as stylex from "@stylexjs/stylex";
import { Router } from "./router";
import { styles } from "./styles.stylex";

function Layout(props: ParentProps) {
  return (
    <div {...stylex.attrs(styles.page)}>
      <div {...stylex.attrs(styles.shell)}>
        <header {...stylex.attrs(styles.header)}>
          <a {...stylex.attrs(styles.brand)} href="/" aria-label="Trendinary home">
            <span {...stylex.attrs(styles.brandMark)}>tr</span>
            <span>trendinary</span>
          </a>
          <nav {...stylex.attrs(styles.nav)} aria-label="Primary navigation">
            <a {...stylex.attrs(styles.navLink)} href="/">Now</a>
            <a {...stylex.attrs(styles.navLink)} href="/peep">PEEP 👀</a>
            <a {...stylex.attrs(styles.navLink)} href="/fomo">FOMO</a>
            <a {...stylex.attrs(styles.navLink)} href="/following">Following</a>
          </nav>
          <a {...stylex.attrs(styles.headerAction)} href="/trend/at-protocol">Search / Ask</a>
        </header>
        <main {...stylex.attrs(styles.main)}>{props.children}</main>
        <footer {...stylex.attrs(styles.footer)}><span>The live dictionary of the internet.</span><span>SCAN · VIBE · WTF? · LORE · PEEP · FOMO</span></footer>
      </div>
    </div>
  );
}

export default function App() {
  return <Router>{(props) => <Layout>{props.children}</Layout>}</Router>;
}
