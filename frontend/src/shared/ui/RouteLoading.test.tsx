import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { RouteLoading } from "./RouteLoading";

describe("RouteLoading", () => {
  afterEach(cleanup);

  it("announces the deferred page being prepared", () => {
    render(<RouteLoading label="library" />);

    expect(screen.getByRole("status")).toHaveTextContent("Loading library");
    expect(screen.getByRole("status")).toHaveAttribute("aria-busy", "true");
  });
});
