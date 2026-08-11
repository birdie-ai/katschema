#!/usr/bin/env node

const fs = require("fs");
const ohm = require("ohm-js");

if (process.argv.length < 4) {
  console.error(
    "Usage: node check.js <grammar.ohm> <example1> [example2 ...]"
  );
  process.exit(2);
}

const grammarPath = process.argv[2];
const examplePaths = process.argv.slice(3);

const grammar = ohm.grammar(fs.readFileSync(grammarPath, "utf8"));

let failed = false;
for (const path of examplePaths) {
  process.stdout.write(`Checking ${path}... `);

  const input = fs.readFileSync(path, "utf8");
  const result = grammar.match(input);

  if (result.failed()) {
    console.log("FAIL");
    console.error(result.message);
    failed = true;
  } else {
    console.log("OK");
  }
}

if (failed) {
  process.exit(1);
}

console.log(`\n✓ Validated ${examplePaths.length} example(s).`);
