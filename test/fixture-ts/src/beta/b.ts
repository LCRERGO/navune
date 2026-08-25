import { normalize } from "../gamma/c";

export function process(v: number): number {
  if (v > 100) {
    return normalize(v) + 1;
  }
  if (v > 50) {
    return normalize(v);
  }
  return normalize(v - 1);
}
