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

test("schema sums and metadata scope", async (t) => {
  const testcases = [
    { name: "inner sum", src: "(string | int)", valid: true },
    { name: "many inner alternatives", src: "(string | int | bool)", valid: true },
    { name: "whole sum metadata", src: '(string | int, optional, label="Value")', valid: true },
    { name: "outer sum", src: "(string) | (int)", valid: true },
    { name: "many outer alternatives", src: "(string) | (int) | (bool)", valid: true },
    { name: "branch metadata", src: '(string, label="Text") | (int, label="Number")', valid: true },
    { name: "wrapped sum metadata", src: '((string, label="Text") | (int, label="Number"), optional)', valid: true },
    { name: "nested inner sum", src: "((string | int) | bool)", valid: true },
    { name: "nested outer sums", src: "(string | int) | (bool | real)", valid: true },
    { name: "structural alternatives", src: '({"name":(string)} | [(int)])', valid: true },
    { name: "references", src: "(pkg.Type | .types.other)", valid: true },
    { name: "sum in list", src: "[(string) | (int)]", valid: true },
    { name: "sum in object", src: '{"value":(string | int, optional)}', valid: true },
    { name: "comments", src: "(string /* left */ | // separator\n int, /* flag */ optional)", valid: true },
    { name: "whole sum constraint syntax", src: "(string | int, x > 0)", valid: true },
    { name: "branch constraint syntax", src: "(string, len(x) > 0) | (int, x > 0)", valid: true },
    { name: "bare types", src: "string | int", valid: false },
    { name: "literal alternatives", src: "1 | 2", valid: false },
    { name: "bare null alternative", src: "(string) | null", valid: false },
    { name: "inner null alternative", src: "(string | null)", valid: false },
    { name: "null type", src: "(null)", valid: false },
    { name: "literal type", src: "(10)", valid: false },
    { name: "literal inner alternative", src: '(string | "text")', valid: false },
    { name: "missing first alternative", src: "(| int)", valid: false },
    { name: "missing inner alternative", src: "(string |)", valid: false },
    { name: "missing outer alternative", src: "(string) |", valid: false },
    { name: "bare outer alternative", src: "(string) | int", valid: false },
    { name: "logical inner separator", src: "(string || int)", valid: false },
    { name: "logical outer separator", src: "(string) || (int)", valid: false },
    { name: "repeated separator", src: "(string | | int)", valid: false },
    { name: "alternative after metadata", src: "(string, optional | int)", valid: false },
    { name: "nested single schema", src: "((int))", valid: false },
    { name: "nested single sum schema", src: "((string | int), optional)", valid: false },
  ];
  for (const tc of testcases) {
    await t.test(tc.name, () => {
      const result = grammar.match(tc.src);
      assert.equal(result.succeeded(), tc.valid, result.message || tc.src);
    });
  }
});

test("schema clause separators", async (t) => {
  const testcases = [
    { name: "type trailing comma", src: "(int,)", valid: true },
    { name: "flag trailing comma", src: "(int, pk,)", valid: true },
    { name: "assignment trailing comma", src: '(string, label="Name",)', valid: true },
    { name: "sum trailing comma", src: "(string | int, optional,)", valid: true },
    { name: "trailing comment", src: "(string | int, optional, /* end */)", valid: true },
    { name: "empty schema", src: "(,)", valid: false },
    { name: "empty first clause", src: "(int,,pk)", valid: false },
    { name: "empty final clause", src: "(int,pk,,)", valid: false },
    { name: "missing assigned value", src: "(int,label=)", valid: false },
    { name: "missing alternative before clause", src: "(int |, optional)", valid: false },
  ];
  for (const tc of testcases) {
    await t.test(tc.name, () => {
      const result = grammar.match(tc.src);
      assert.equal(result.succeeded(), tc.valid, result.message || tc.src);
    });
  }
});

test("constraint expressions", async (t) => {
  const testcases = [
    { name: "arithmetic", src: "(int, x + 2 * 3 - 4 / 2 % 2 > 0)", valid: true },
    { name: "range chain", src: "(int, 0 <= x < 100)", valid: true },
    { name: "grouped logical expression", src: "(int, (x > 0 && x != 10) || x == -1)", valid: true },
    { name: "unary expressions", src: "(int, !false && -x < +10)", valid: true },
    { name: "repeated unary", src: "(int, !!true && --x >= 0)", valid: true },
    { name: "separated negative bound", src: "(int, x >= - /* sign */ 10)", valid: true },
    { name: "membership", src: "(int, x in /* values */ [1,2,3])", valid: true },
    { name: "string match", src: '(string, x ~ "^[a-z]+$")', valid: true },
    { name: "zero arguments", src: "(int, ready())", valid: true },
    { name: "trailing argument comma", src: "(int, check(x, 1,))", valid: true },
    { name: "nested calls", src: "(int, check(x + 1, pkg.limit(),))", valid: true },
    { name: "qualified call", src: "(string, pkg.check(x))", valid: true },
    { name: "path call", src: "(string, .types.check(x))", valid: true },
    { name: "keyword reference component", src: "(int, pkg.true(x) && .null == null)", valid: true },
    { name: "path comparison", src: "(int, .limits.minimum <= x)", valid: true },
    { name: "object field constraints", src: '(myObjType, len(x.tags) > 0, x.name != "")', valid: true },
    { name: "grouped comparisons", src: "(int, (x == 1) == true)", valid: true },
    { name: "mixed clauses", src: '(int, label="Age", x > 0, optional)', valid: true },
    { name: "expression name prefix", src: "(int, intrue(x))", valid: true },
    { name: "metadata collection", src: "(int, choices=[(string | int)])", valid: true },
    { name: "whole sum constraint syntax", src: "(string | int, x > 0)", valid: true },
    { name: "repeated equality", src: "(int, x == 1 == 2)", valid: false },
    { name: "mixed equality operators", src: "(int, x == 1 != 2)", valid: false },
    { name: "repeated membership", src: "(int, x in [1] in [2])", valid: false },
    { name: "mixed membership and equality", src: "(int, x in [1] == true)", valid: false },
    { name: "repeated match", src: '(string, x ~ "a" ~ "b")', valid: false },
    { name: "missing binary operand", src: "(int, x >)", valid: false },
    { name: "missing unary operand", src: "(int, !)", valid: false },
    { name: "missing expression separator", src: "(int, x > 0 optional)", valid: false },
    { name: "empty group", src: "(int, ())", valid: false },
    { name: "group comma", src: "(int, (x,))", valid: false },
    { name: "missing closing group", src: "(int, (x > 0)", valid: false },
    { name: "call comma without arguments", src: "(int, check(,))", valid: false },
    { name: "missing argument", src: "(int, check(x,,1))", valid: false },
    { name: "missing argument comma", src: "(int, check(x 1))", valid: false },
    { name: "multiple trailing argument commas", src: "(int, check(x,,))", valid: false },
    { name: "calling a group", src: "(int, (check)(x))", valid: false },
    { name: "calling null", src: "(int, null())", valid: false },
    { name: "reserved reference start", src: "(int, true.field == 1)", valid: false },
    { name: "reserved metadata name", src: "(int, true=1)", valid: false },
    { name: "assigned expression", src: "(int, offset=1+2)", valid: false },
    { name: "assigned schema", src: "(int, value=(string))", valid: false },
    { name: "membership keyword boundary", src: "(int, x intrue)", valid: false },
    { name: "membership underscore boundary", src: "(int, x in_values)", valid: false },
    { name: "literal keyword boundary", src: "(int, truein [true])", valid: false },
    { name: "type pipe in expression", src: "(int, x > 0 | x < 10)", valid: false },
    { name: "malformed equality", src: "(int, x === 1)", valid: false },
  ];
  for (const tc of testcases) {
    await t.test(tc.name, () => {
      const result = grammar.match(tc.src);
      assert.equal(result.succeeded(), tc.valid, result.message || tc.src);
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
