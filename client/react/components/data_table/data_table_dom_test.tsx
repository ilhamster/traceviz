import React from "react";
import { render, fireEvent } from "@testing-library/react";
import { MantineProvider } from "@mantine/core";
import {
  Action,
  AppCore,
  GlobalRef,
  Interactions,
  LocalValue,
  Set,
  StringValue,
  node,
  str,
  strs,
  valueMap,
} from "@traceviz/client-core";
import type { ResponseNode } from "@traceviz/client-core";
import { DataTable } from "./data_table.tsx";
import { AppCoreContext } from "../../core/index.ts";

const tableData: ResponseNode = node(
  valueMap(),
  node(
    valueMap(),
    node(
      valueMap(
        { key: "category_defined_id", val: str("name") },
        { key: "category_display_name", val: str("Name") },
        {
          key: "category_description",
          val: str("Give this name to order this scoop!"),
        },
      ),
    ),
    node(
      valueMap(
        { key: "category_defined_id", val: str("color") },
        { key: "category_display_name", val: str("Color") },
        {
          key: "category_description",
          val: str("How will you recognize it?"),
        },
      ),
    ),
    node(
      valueMap(
        { key: "category_defined_id", val: str("flavor") },
        { key: "category_display_name", val: str("Flavor") },
        {
          key: "category_description",
          val: str("What will it taste like?"),
        },
      ),
    ),
    node(
      valueMap(
        { key: "category_defined_id", val: str("label") },
        { key: "category_display_name", val: str("Label") },
        {
          key: "category_description",
          val: str("This ice cream's label"),
        },
      ),
    ),
  ),
  node(
    valueMap({ key: "flavor", val: str("vanilla") }),
    node(
      valueMap(
        { key: "category_ids", val: strs("name") },
        { key: "table_cell", val: str("vanilla") },
      ),
    ),
    node(
      valueMap(
        { key: "category_ids", val: strs("color") },
        { key: "table_cell", val: str("white") },
      ),
    ),
    node(
      valueMap(
        { key: "category_ids", val: strs("flavor") },
        { key: "table_cell", val: str("vanilla-y") },
      ),
    ),
    node(
      valueMap(
        { key: "category_ids", val: strs("label") },
        { key: "name", val: str("vanilla") },
        { key: "color", val: str("white") },
        { key: "table_formatted_cell", val: str("$(name) ($(color))") },
      ),
    ),
  ),
  node(
    valueMap({ key: "flavor", val: str("chocolate") }),
    node(
      valueMap(
        { key: "category_ids", val: strs("name") },
        { key: "table_cell", val: str("chocolate") },
      ),
    ),
    node(
      valueMap(
        { key: "category_ids", val: strs("color") },
        { key: "table_cell", val: str("brown") },
      ),
    ),
    node(
      valueMap(
        { key: "category_ids", val: strs("flavor") },
        { key: "table_cell", val: str("chocolatey") },
      ),
    ),
    node(
      valueMap(
        { key: "category_ids", val: strs("label") },
        { key: "name", val: str("chocolate") },
        { key: "color", val: str("brown") },
        { key: "table_formatted_cell", val: str("$(name) ($(color))") },
      ),
    ),
  ),
  node(
    valueMap({ key: "flavor", val: str("strawberry") }),
    node(
      valueMap(
        { key: "category_ids", val: strs("name") },
        { key: "table_cell", val: str("strawberry") },
      ),
    ),
    node(
      valueMap(
        { key: "category_ids", val: strs("color") },
        { key: "table_cell", val: str("pink") },
      ),
    ),
    node(
      valueMap(
        { key: "category_ids", val: strs("flavor") },
        { key: "table_cell", val: str("berrylicious") },
      ),
    ),
    node(
      valueMap(
        { key: "category_ids", val: strs("label") },
        { key: "name", val: str("strawberry") },
        { key: "color", val: str("pink") },
        { key: "table_formatted_cell", val: str("$(name) ($(color))") },
      ),
    ),
  ),
);

describe("DataTable", () => {
  it("renders rows and supports row click interactions", () => {
    const appCore = new AppCore();
    const errors: string[] = [];
    const errSub = appCore.configurationErrors.subscribe((err) => {
      errors.push(err.toString());
    });
    appCore.publish();

    const selectedFlavor = new StringValue("");
    appCore.globalState.set("selected_flavor", selectedFlavor);

    const interactions = new Interactions().withAction(
      new Action("rows", "click", [
        new Set(
          new GlobalRef(appCore, "selected_flavor"),
          new LocalValue("flavor"),
        ),
      ]),
    );

    const { container } = render(
      <MantineProvider>
        <AppCoreContext.Provider value={appCore}>
          <DataTable data={tableData} interactions={interactions} />
        </AppCoreContext.Provider>
      </MantineProvider>,
    );

    const headerCells = Array.from(container.querySelectorAll("thead th")).map(
      (el) => el.textContent?.trim(),
    );
    expect(headerCells).toEqual(["Name", "Color", "Flavor", "Label"]);

    const bodyRows = Array.from(container.querySelectorAll("tbody tr"));
    expect(bodyRows.length).toBe(3);

    const bodyValues = bodyRows.map((row) =>
      Array.from(row.querySelectorAll("td")).map((cell) =>
        cell.textContent?.trim(),
      ),
    );
    expect(bodyValues).toEqual([
      ["vanilla", "white", "vanilla-y", "vanilla (white)"],
      ["chocolate", "brown", "chocolatey", "chocolate (brown)"],
      ["strawberry", "pink", "berrylicious", "strawberry (pink)"],
    ]);

    fireEvent.click(bodyRows[0]);
    expect(selectedFlavor.val).toBe("vanilla");
    fireEvent.click(bodyRows[2]);
    expect(selectedFlavor.val).toBe("strawberry");

    expect(errors).toEqual([]);
    errSub.unsubscribe();
  });
});
