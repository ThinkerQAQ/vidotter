"use strict";

const assert = require("node:assert/strict");
const media = require("../core/media.js");
const store = require("../core/store.js");

const at = 1793500590 * 1000;
assert.equal(media.mediaExpiryMs("https://media.test/master.m3u8?expires=1793500557"),
  1793500557 * 1000);
assert.equal(media.mediaExpiryMs("https://other.test/movie.mp4?EXP=1793500557000"),
  1793500557000);
assert.equal(media.mediaExpiryMs("https://media.test/movie.mp4?expiry=1793500557"),
  1793500557000);
assert.equal(media.mediaExpiryMs("https://media.test/movie.mp4?token=anything"), 0);
assert.equal(media.mediaExpiryMs("https://media.test/movie.mp4?exp=300"), 0);
assert.equal(media.mediaExpiryMs("https://media.test/movie.mp4?exp=not-a-date"), 0);
assert.equal(media.mediaExpiryMs("https://media.test/movie.mp4?expires=2026-11-01T02%3A35%3A57Z"),
  1793500557000);
assert.equal(media.mediaExpiring({url:"https://media.test/test.mp4?exp=1793500557"}, at), true);
assert.equal(media.mediaExpiring({url:"https://media.test/test.mp4?exp=1793500700"}, at), false);
assert.equal(media.mediaExpiring({url:"https://media.test/test.mp4?unknown=1793500557"}, at), false);

const expired = media.makeResource({url:"https://media.test/expired.m3u8?exp=1793500557",
  tabId: 1, detectedAt: at - 1000});
assert.equal(media.recentResourcesForSync([expired], {}, at).length, 0);
const unknown = media.makeResource({url:"https://media.test/master.m3u8?opaque=token",
  tabId: 1, detectedAt: at - 1000});
assert.equal(media.recentResourcesForSync([unknown], {}, at).length, 1);

const db = new Map();
global.chrome = {
  storage: { session: {
    async get(key) {return {[key]:db.get(key)};},
    async set(value) {for (const [key, data] of Object.entries(value)) db.set(key, data);},
    async remove(key) {db.delete(key);}
  }}
};

(async () => {
  const now = Date.now();
  const first = media.makeResource({url:"https://media.test/stream.m3u8", tabId: 5,
    detectedAt: now - 5000});
  await store.put(first);
  await store.put(first); // reconnection must not reset capture time
  assert.equal((await store.get(5))[0].detectedAt, first.detectedAt);
  await store.put({...first, detectedAt:now - 1000}); // real new network request
  assert.equal((await store.get(5))[0].detectedAt, now - 1000);
  await store.put({...first, detectedAt:now - 7000}); // out-of-order replay
  assert.equal((await store.get(5))[0].detectedAt, now - 1000);
  console.log("media expiry and capture freshness tests passed");
})().catch(err => { console.error(err); process.exitCode = 1; });
