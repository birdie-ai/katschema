const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { test } = require("node:test");
const ohm = require("ohm-js");

const grammar = ohm.grammar(
  fs.readFileSync(path.join(__dirname, "..", "grammar.ohm"), "utf8")
);

test("collection separators", async (t) => {
  const testcases = [
    { name: "empty", src: "", valid: true },
    { name: "empty with comment", src: "/* empty */", valid: true },
    { name: "one value", src: "1", valid: true },
    { name: "multiple values", src: '1,true,null,"a",{},[]', valid: true },
    { name: "trailing comma", src: "1,", valid: true },
    { name: "multiple values and trailing comma", src: "1,true,", valid: true },
    { name: "schemas", src: "(int),(string),", valid: true },
    { name: "comments around comma", src: "1/*a*/,//b\ntrue", valid: true },
    { name: "comment after trailing comma", src: "1,/*end*/", valid: true },
    { name: "leading comma", src: ",1", valid: false },
    { name: "comma without values", src: ",", valid: false },
    { name: "missing value", src: "1,,2", valid: false },
    { name: "multiple trailing commas", src: "1,,", valid: false },
    { name: "comment instead of value", src: "1,/*gap*/,2", valid: false },
    { name: "number and keyword", src: "1true", valid: false },
    { name: "number and identifier", src: "1foo", valid: false },
    { name: "adjacent values after comma", src: "0,1true", valid: false },
    { name: "identifier after comma", src: "0,1foo", valid: false },
    { name: "whitespace instead of comma", src: "0,1 true", valid: false },
    { name: "comment instead of comma", src: "0,1/**/true", valid: false },
    { name: "adjacent strings", src: '0,"a""b"', valid: false },
    { name: "adjacent objects", src: "0,{}{}", valid: false },
    { name: "adjacent arrays", src: "0,[][]", valid: false },
    { name: "adjacent schemas", src: "0,(int)(string)", valid: false },
    { name: "leading zero", src: "0,01", valid: false },
    { name: "missing fraction", src: "0,1.", valid: false },
    { name: "missing exponent", src: "0,1e", valid: false },
  ];

  for (const delimiters of [
    { open: "[", close: "]" },
    { open: "#{", close: "}" },
  ]) {
    for (const tc of testcases) {
      await t.test(`${delimiters.open} ${tc.name}`, () => {
        const src = delimiters.open + tc.src + delimiters.close;
        const result = grammar.match(src);
        assert.equal(result.succeeded(), tc.valid, result.message || src);
      });
    }
  }
});

test("one complete value", async (t) => {
  for (const src of ["1true", "1foo", "1 true", '"a""b"', "{}{}", "[][]"]) {
    await t.test(src, () => {
      assert.equal(grammar.match(src).succeeded(), false);
    });
  }
});

test("katschema examples", async (t) => {
  const dir = path.join(__dirname, "examples");
  const files = fs.readdirSync(dir).filter((name) => name.endsWith(".ks")).sort();
  assert.ok(files.length > 0);
  for (const file of files) {
    await t.test(file, () => {
      const result = grammar.match(fs.readFileSync(path.join(dir, file), "utf8"));
      assert.ok(result.succeeded(), result.message);
    });
  }
});

test("valid JSON fixtures", async (t) => {
  const dir = path.join(__dirname, "..", "parser", "testdata", "json");
  const files = fs.readdirSync(dir).filter((name) => name.endsWith(".json")).sort();
  assert.ok(files.length > 0);
  for (const file of files) {
    await t.test(file, () => {
      const src = fs.readFileSync(path.join(dir, file), "utf8");
      JSON.parse(src);
      const result = grammar.match(src);
      assert.ok(result.succeeded(), result.message);
    });
  }
});
