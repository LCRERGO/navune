import { Service } from "../alpha/a";
import { process } from "../beta/b";

export interface Config {
  threshold: number;
}

export function normalize(v: number): number {
  if (v < 0) {
    return 0;
  }
  return v;
}

export function load(): Config {
  const s = new Service();
  process(1);
  return { threshold: s ? 10 : 0 };
}
