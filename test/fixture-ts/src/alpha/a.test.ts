import { helper } from "./a";

describe("helper", () => {
  it("sums evens", () => {
    const got = helper(6);
    if (got !== 6) {
      throw new Error("want 6");
    }
  });
});
