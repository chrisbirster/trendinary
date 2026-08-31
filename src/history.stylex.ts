import * as stylex from "@stylexjs/stylex";
import { colors } from "./styles.stylex";

export const historyStyles = stylex.create({
  panel: {
    border: `1px solid ${colors.line}`,
    borderRadius: "20px",
    backgroundColor: colors.panel,
    padding: "20px",
  },
  chart: {
    height: "148px",
    display: "flex",
    alignItems: "end",
    gap: "4px",
    paddingTop: "20px",
    borderBottom: `1px solid ${colors.line}`,
    overflow: "hidden",
  },
  barSlot: {
    height: "100%",
    minWidth: "4px",
    flex: 1,
    display: "flex",
    alignItems: "end",
  },
  bar: {
    width: "100%",
    minHeight: "4px",
    borderRadius: "4px 4px 0 0",
    backgroundColor: colors.acid,
    transition: "height 180ms ease",
  },
  footer: {
    display: "flex",
    justifyContent: "space-between",
    alignItems: "center",
    gap: "12px",
    paddingTop: "10px",
    color: colors.muted,
    fontSize: "11px",
    fontWeight: 800,
  },
  metrics: {
    display: "grid",
    gridTemplateColumns: { default: "repeat(4, minmax(0, 1fr))", "@media (max-width: 720px)": "repeat(2, minmax(0, 1fr))" },
    gap: "8px",
    marginTop: "12px",
  },
  metric: {
    border: `1px solid ${colors.line}`,
    borderRadius: "12px",
    padding: "12px",
    backgroundColor: colors.panel2,
  },
  metricLabel: {
    color: colors.muted,
    fontSize: "10px",
    fontWeight: 900,
    letterSpacing: "0.08em",
    textTransform: "uppercase",
  },
  metricValue: {
    marginTop: "5px",
    fontSize: "21px",
    fontWeight: 950,
    letterSpacing: "-0.04em",
  },
  empty: {
    color: colors.muted,
    fontSize: "13px",
    lineHeight: 1.5,
    paddingBlock: "24px 8px",
  },
});
