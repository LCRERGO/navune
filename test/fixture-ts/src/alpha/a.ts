// src/alpha/a.ts forms a 3-file cycle with beta and gamma.
import { process } from "../beta/b";
import { normalize } from "../gamma/c";
import * as React from "react";

export class Service {
  handle(x: number): number {
    if (x > 10) {
      return process(x);
    }
    return normalize(x);
  }
}

export function helper(p: number): number {
  let sum = 0;
  for (let i = 0; i < p; i++) {
    if (i % 2 === 0) {
      sum += i;
    }
  }
  return sum;
}
