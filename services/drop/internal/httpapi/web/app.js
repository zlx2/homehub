// @__NO_SIDE_EFFECTS__
function ks(e) {
  const t = /* @__PURE__ */ Object.create(null);
  for (const n of e.split(",")) t[n] = 1;
  return (n) => n in t;
}
const ce = {}, Nt = [], Ge = () => {
}, Vi = () => !1, Wn = (e) => e.charCodeAt(0) === 111 && e.charCodeAt(1) === 110 && // uppercase letter
(e.charCodeAt(2) > 122 || e.charCodeAt(2) < 97), qn = (e) => e.startsWith("onUpdate:"), me = Object.assign, Ls = (e, t) => {
  const n = e.indexOf(t);
  n > -1 && e.splice(n, 1);
}, ro = Object.prototype.hasOwnProperty, oe = (e, t) => ro.call(e, t), U = Array.isArray, Ht = (e) => bn(e) === "[object Map]", ji = (e) => bn(e) === "[object Set]", Qs = (e) => bn(e) === "[object Date]", X = (e) => typeof e == "function", he = (e) => typeof e == "string", Je = (e) => typeof e == "symbol", le = (e) => e !== null && typeof e == "object", Ki = (e) => (le(e) || X(e)) && X(e.then) && X(e.catch), Ui = Object.prototype.toString, bn = (e) => Ui.call(e), oo = (e) => bn(e).slice(8, -1), Wi = (e) => bn(e) === "[object Object]", Ps = (e) => he(e) && e !== "NaN" && e[0] !== "-" && "" + parseInt(e, 10) === e, sn = /* @__PURE__ */ ks(
  // the leading comma is intentional so empty string "" is also included
  ",key,ref,ref_for,ref_key,onVnodeBeforeMount,onVnodeMounted,onVnodeBeforeUpdate,onVnodeUpdated,onVnodeBeforeUnmount,onVnodeUnmounted"
), Yn = (e) => {
  const t = /* @__PURE__ */ Object.create(null);
  return ((n) => t[n] || (t[n] = e(n)));
}, lo = /-\w/g, He = Yn(
  (e) => e.replace(lo, (t) => t.slice(1).toUpperCase())
), ao = /\B([A-Z])/g, It = Yn(
  (e) => e.replace(ao, "-$1").toLowerCase()
), qi = Yn((e) => e.charAt(0).toUpperCase() + e.slice(1)), ss = Yn(
  (e) => e ? `on${qi(e)}` : ""
), ze = (e, t) => !Object.is(e, t), kn = (e, ...t) => {
  for (let n = 0; n < e.length; n++)
    e[n](...t);
}, Yi = (e, t, n, s = !1) => {
  Object.defineProperty(e, t, {
    configurable: !0,
    enumerable: !1,
    writable: s,
    value: n
  });
}, Is = (e) => {
  const t = parseFloat(e);
  return isNaN(t) ? e : t;
}, uo = (e) => {
  const t = he(e) ? Number(e) : NaN;
  return isNaN(t) ? e : t;
};
let ei;
const Xn = () => ei || (ei = typeof globalThis < "u" ? globalThis : typeof self < "u" ? self : typeof window < "u" ? window : typeof global < "u" ? global : {});
function Kt(e) {
  if (U(e)) {
    const t = {};
    for (let n = 0; n < e.length; n++) {
      const s = e[n], i = he(s) ? po(s) : Kt(s);
      if (i)
        for (const r in i)
          t[r] = i[r];
    }
    return t;
  } else if (he(e) || le(e))
    return e;
}
const co = /;(?![^(]*\))/g, fo = /:([^]+)/, ho = /\/\*[^]*?\*\//g;
function po(e) {
  const t = {};
  return e.replace(ho, "").split(co).forEach((n) => {
    if (n) {
      const s = n.split(fo);
      s.length > 1 && (t[s[0].trim()] = s[1].trim());
    }
  }), t;
}
function Ae(e) {
  let t = "";
  if (he(e))
    t = e;
  else if (U(e))
    for (let n = 0; n < e.length; n++) {
      const s = Ae(e[n]);
      s && (t += s + " ");
    }
  else if (le(e))
    for (const n in e)
      e[n] && (t += n + " ");
  return t.trim();
}
const go = "itemscope,allowfullscreen,formnovalidate,ismap,nomodule,novalidate,readonly", mo = /* @__PURE__ */ ks(go);
function Xi(e) {
  return !!e || e === "";
}
function vo(e, t) {
  if (e.length !== t.length) return !1;
  let n = !0;
  for (let s = 0; n && s < e.length; s++)
    n = Os(e[s], t[s]);
  return n;
}
function Os(e, t) {
  if (e === t) return !0;
  let n = Qs(e), s = Qs(t);
  if (n || s)
    return n && s ? e.getTime() === t.getTime() : !1;
  if (n = Je(e), s = Je(t), n || s)
    return e === t;
  if (n = U(e), s = U(t), n || s)
    return n && s ? vo(e, t) : !1;
  if (n = le(e), s = le(t), n || s) {
    if (!n || !s)
      return !1;
    const i = Object.keys(e).length, r = Object.keys(t).length;
    if (i !== r)
      return !1;
    for (const o in e) {
      const l = e.hasOwnProperty(o), a = t.hasOwnProperty(o);
      if (l && !a || !l && a || !Os(e[o], t[o]))
        return !1;
    }
  }
  return String(e) === String(t);
}
const zi = (e) => !!(e && e.__v_isRef === !0), ne = (e) => he(e) ? e : e == null ? "" : U(e) || le(e) && (e.toString === Ui || !X(e.toString)) ? zi(e) ? ne(e.value) : JSON.stringify(e, Gi, 2) : String(e), Gi = (e, t) => zi(t) ? Gi(e, t.value) : Ht(t) ? {
  [`Map(${t.size})`]: [...t.entries()].reduce(
    (n, [s, i], r) => (n[is(s, r) + " =>"] = i, n),
    {}
  )
} : ji(t) ? {
  [`Set(${t.size})`]: [...t.values()].map((n) => is(n))
} : Je(t) ? is(t) : le(t) && !U(t) && !Wi(t) ? String(t) : t, is = (e, t = "") => {
  var n;
  return (
    // Symbol.description in es2019+ so we need to cast here to pass
    // the lib: es2016 check
    Je(e) ? `Symbol(${(n = e.description) != null ? n : t})` : e
  );
};
let be;
class yo {
  // TODO isolatedDeclarations "__v_skip"
  constructor(t = !1) {
    this.detached = t, this._active = !0, this._on = 0, this.effects = [], this.cleanups = [], this._isPaused = !1, this._warnOnRun = !0, this.__v_skip = !0, !t && be && (be.active ? (this.parent = be, this.index = (be.scopes || (be.scopes = [])).push(
      this
    ) - 1) : (this._active = !1, this._warnOnRun = !1));
  }
  get active() {
    return this._active;
  }
  pause() {
    if (this._active) {
      this._isPaused = !0;
      let t, n;
      if (this.scopes)
        for (t = 0, n = this.scopes.length; t < n; t++)
          this.scopes[t].pause();
      for (t = 0, n = this.effects.length; t < n; t++)
        this.effects[t].pause();
    }
  }
  /**
   * Resumes the effect scope, including all child scopes and effects.
   */
  resume() {
    if (this._active && this._isPaused) {
      this._isPaused = !1;
      let t, n;
      if (this.scopes)
        for (t = 0, n = this.scopes.length; t < n; t++)
          this.scopes[t].resume();
      for (t = 0, n = this.effects.length; t < n; t++)
        this.effects[t].resume();
    }
  }
  run(t) {
    if (this._active) {
      const n = be;
      try {
        return be = this, t();
      } finally {
        be = n;
      }
    }
  }
  /**
   * This should only be called on non-detached scopes
   * @internal
   */
  on() {
    ++this._on === 1 && (this.prevScope = be, be = this);
  }
  /**
   * This should only be called on non-detached scopes
   * @internal
   */
  off() {
    if (this._on > 0 && --this._on === 0) {
      if (be === this)
        be = this.prevScope;
      else {
        let t = be;
        for (; t; ) {
          if (t.prevScope === this) {
            t.prevScope = this.prevScope;
            break;
          }
          t = t.prevScope;
        }
      }
      this.prevScope = void 0;
    }
  }
  stop(t) {
    if (this._active) {
      this._active = !1;
      let n, s;
      for (n = 0, s = this.effects.length; n < s; n++)
        this.effects[n].stop();
      for (this.effects.length = 0, n = 0, s = this.cleanups.length; n < s; n++)
        this.cleanups[n]();
      if (this.cleanups.length = 0, this.scopes) {
        for (n = 0, s = this.scopes.length; n < s; n++)
          this.scopes[n].stop(!0);
        this.scopes.length = 0;
      }
      if (!this.detached && this.parent && !t) {
        const i = this.parent.scopes.pop();
        i && i !== this && (this.parent.scopes[this.index] = i, i.index = this.index);
      }
      this.parent = void 0;
    }
  }
}
function bo() {
  return be;
}
let fe;
const rs = /* @__PURE__ */ new WeakSet();
class Ji {
  constructor(t) {
    this.fn = t, this.deps = void 0, this.depsTail = void 0, this.flags = 5, this.next = void 0, this.cleanup = void 0, this.scheduler = void 0, be && (be.active ? be.effects.push(this) : this.flags &= -2);
  }
  pause() {
    this.flags |= 64;
  }
  resume() {
    this.flags & 64 && (this.flags &= -65, rs.has(this) && (rs.delete(this), this.trigger()));
  }
  /**
   * @internal
   */
  notify() {
    this.flags & 2 && !(this.flags & 32) || this.flags & 8 || Qi(this);
  }
  run() {
    if (!(this.flags & 1))
      return this.fn();
    this.flags |= 2, ti(this), er(this);
    const t = fe, n = Be;
    fe = this, Be = !0;
    try {
      return this.fn();
    } finally {
      tr(this), fe = t, Be = n, this.flags &= -3;
    }
  }
  stop() {
    if (this.flags & 1) {
      for (let t = this.deps; t; t = t.nextDep)
        Rs(t);
      this.deps = this.depsTail = void 0, ti(this), this.onStop && this.onStop(), this.flags &= -2;
    }
  }
  trigger() {
    this.flags & 64 ? rs.add(this) : this.scheduler ? this.scheduler() : this.runIfDirty();
  }
  /**
   * @internal
   */
  runIfDirty() {
    ms(this) && this.run();
  }
  get dirty() {
    return ms(this);
  }
}
let Zi = 0, rn, on;
function Qi(e, t = !1) {
  if (e.flags |= 8, t) {
    e.next = on, on = e;
    return;
  }
  e.next = rn, rn = e;
}
function Ds() {
  Zi++;
}
function Fs() {
  if (--Zi > 0)
    return;
  if (on) {
    let t = on;
    for (on = void 0; t; ) {
      const n = t.next;
      t.next = void 0, t.flags &= -9, t = n;
    }
  }
  let e;
  for (; rn; ) {
    let t = rn;
    for (rn = void 0; t; ) {
      const n = t.next;
      if (t.next = void 0, t.flags &= -9, t.flags & 1)
        try {
          t.trigger();
        } catch (s) {
          e || (e = s);
        }
      t = n;
    }
  }
  if (e) throw e;
}
function er(e) {
  for (let t = e.deps; t; t = t.nextDep)
    t.version = -1, t.prevActiveLink = t.dep.activeLink, t.dep.activeLink = t;
}
function tr(e) {
  let t, n = e.depsTail, s = n;
  for (; s; ) {
    const i = s.prevDep;
    s.version === -1 ? (s === n && (n = i), Rs(s), wo(s)) : t = s, s.dep.activeLink = s.prevActiveLink, s.prevActiveLink = void 0, s = i;
  }
  e.deps = t, e.depsTail = n;
}
function ms(e) {
  for (let t = e.deps; t; t = t.nextDep)
    if (t.dep.version !== t.version || t.dep.computed && (nr(t.dep.computed) || t.dep.version !== t.version))
      return !0;
  return !!e._dirty;
}
function nr(e) {
  if (e.flags & 4 && !(e.flags & 16) || (e.flags &= -17, e.globalVersion === dn) || (e.globalVersion = dn, !e.isSSR && e.flags & 128 && (!e.deps && !e._dirty || !ms(e))))
    return;
  e.flags |= 2;
  const t = e.dep, n = fe, s = Be;
  fe = e, Be = !0;
  try {
    er(e);
    const i = e.fn(e._value);
    (t.version === 0 || ze(i, e._value)) && (e.flags |= 128, e._value = i, t.version++);
  } catch (i) {
    throw t.version++, i;
  } finally {
    fe = n, Be = s, tr(e), e.flags &= -3;
  }
}
function Rs(e, t = !1) {
  const { dep: n, prevSub: s, nextSub: i } = e;
  if (s && (s.nextSub = i, e.prevSub = void 0), i && (i.prevSub = s, e.nextSub = void 0), n.subs === e && (n.subs = s, !s && n.computed)) {
    n.computed.flags &= -5;
    for (let r = n.computed.deps; r; r = r.nextDep)
      Rs(r, !0);
  }
  !t && !--n.sc && n.map && n.map.delete(n.key);
}
function wo(e) {
  const { prevDep: t, nextDep: n } = e;
  t && (t.nextDep = n, e.prevDep = void 0), n && (n.prevDep = t, e.nextDep = void 0);
}
let Be = !0;
const sr = [];
function Ze() {
  sr.push(Be), Be = !1;
}
function Qe() {
  const e = sr.pop();
  Be = e === void 0 ? !0 : e;
}
function ti(e) {
  const { cleanup: t } = e;
  if (e.cleanup = void 0, t) {
    const n = fe;
    fe = void 0;
    try {
      t();
    } finally {
      fe = n;
    }
  }
}
let dn = 0;
class _o {
  constructor(t, n) {
    this.sub = t, this.dep = n, this.version = n.version, this.nextDep = this.prevDep = this.nextSub = this.prevSub = this.prevActiveLink = void 0;
  }
}
class Ns {
  // TODO isolatedDeclarations "__v_skip"
  constructor(t) {
    this.computed = t, this.version = 0, this.activeLink = void 0, this.subs = void 0, this.map = void 0, this.key = void 0, this.sc = 0, this.__v_skip = !0;
  }
  track(t) {
    if (!fe || !Be || fe === this.computed)
      return;
    let n = this.activeLink;
    if (n === void 0 || n.sub !== fe)
      n = this.activeLink = new _o(fe, this), fe.deps ? (n.prevDep = fe.depsTail, fe.depsTail.nextDep = n, fe.depsTail = n) : fe.deps = fe.depsTail = n, ir(n);
    else if (n.version === -1 && (n.version = this.version, n.nextDep)) {
      const s = n.nextDep;
      s.prevDep = n.prevDep, n.prevDep && (n.prevDep.nextDep = s), n.prevDep = fe.depsTail, n.nextDep = void 0, fe.depsTail.nextDep = n, fe.depsTail = n, fe.deps === n && (fe.deps = s);
    }
    return n;
  }
  trigger(t) {
    this.version++, dn++, this.notify(t);
  }
  notify(t) {
    Ds();
    try {
      for (let n = this.subs; n; n = n.prevSub)
        n.sub.notify() && n.sub.dep.notify();
    } finally {
      Fs();
    }
  }
}
function ir(e) {
  if (e.dep.sc++, e.sub.flags & 4) {
    const t = e.dep.computed;
    if (t && !e.dep.subs) {
      t.flags |= 20;
      for (let s = t.deps; s; s = s.nextDep)
        ir(s);
    }
    const n = e.dep.subs;
    n !== e && (e.prevSub = n, n && (n.nextSub = e)), e.dep.subs = e;
  }
}
const vs = /* @__PURE__ */ new WeakMap(), Lt = /* @__PURE__ */ Symbol(
  ""
), ys = /* @__PURE__ */ Symbol(
  ""
), hn = /* @__PURE__ */ Symbol(
  ""
);
function xe(e, t, n) {
  if (Be && fe) {
    let s = vs.get(e);
    s || vs.set(e, s = /* @__PURE__ */ new Map());
    let i = s.get(n);
    i || (s.set(n, i = new Ns()), i.map = s, i.key = n), i.track();
  }
}
function ut(e, t, n, s, i, r) {
  const o = vs.get(e);
  if (!o) {
    dn++;
    return;
  }
  const l = (a) => {
    a && a.trigger();
  };
  if (Ds(), t === "clear")
    o.forEach(l);
  else {
    const a = U(e), d = a && Ps(n);
    if (a && n === "length") {
      const c = Number(s);
      o.forEach((h, v) => {
        (v === "length" || v === hn || !Je(v) && v >= c) && l(h);
      });
    } else
      switch ((n !== void 0 || o.has(void 0)) && l(o.get(n)), d && l(o.get(hn)), t) {
        case "add":
          a ? d && l(o.get("length")) : (l(o.get(Lt)), Ht(e) && l(o.get(ys)));
          break;
        case "delete":
          a || (l(o.get(Lt)), Ht(e) && l(o.get(ys)));
          break;
        case "set":
          Ht(e) && l(o.get(Lt));
          break;
      }
  }
  Fs();
}
function Dt(e) {
  const t = /* @__PURE__ */ re(e);
  return t === e ? t : (xe(t, "iterate", hn), /* @__PURE__ */ Fe(e) ? t : t.map(Ve));
}
function zn(e) {
  return xe(e = /* @__PURE__ */ re(e), "iterate", hn), e;
}
function Ye(e, t) {
  return /* @__PURE__ */ dt(e) ? jt(/* @__PURE__ */ Pt(e) ? Ve(t) : t) : Ve(t);
}
const xo = {
  __proto__: null,
  [Symbol.iterator]() {
    return os(this, Symbol.iterator, (e) => Ye(this, e));
  },
  concat(...e) {
    return Dt(this).concat(
      ...e.map((t) => U(t) ? Dt(t) : t)
    );
  },
  entries() {
    return os(this, "entries", (e) => (e[1] = Ye(this, e[1]), e));
  },
  every(e, t) {
    return rt(this, "every", e, t, void 0, arguments);
  },
  filter(e, t) {
    return rt(
      this,
      "filter",
      e,
      t,
      (n) => n.map((s) => Ye(this, s)),
      arguments
    );
  },
  find(e, t) {
    return rt(
      this,
      "find",
      e,
      t,
      (n) => Ye(this, n),
      arguments
    );
  },
  findIndex(e, t) {
    return rt(this, "findIndex", e, t, void 0, arguments);
  },
  findLast(e, t) {
    return rt(
      this,
      "findLast",
      e,
      t,
      (n) => Ye(this, n),
      arguments
    );
  },
  findLastIndex(e, t) {
    return rt(this, "findLastIndex", e, t, void 0, arguments);
  },
  // flat, flatMap could benefit from ARRAY_ITERATE but are not straight-forward to implement
  forEach(e, t) {
    return rt(this, "forEach", e, t, void 0, arguments);
  },
  includes(...e) {
    return ls(this, "includes", e);
  },
  indexOf(...e) {
    return ls(this, "indexOf", e);
  },
  join(e) {
    return Dt(this).join(e);
  },
  // keys() iterator only reads `length`, no optimization required
  lastIndexOf(...e) {
    return ls(this, "lastIndexOf", e);
  },
  map(e, t) {
    return rt(this, "map", e, t, void 0, arguments);
  },
  pop() {
    return Xt(this, "pop");
  },
  push(...e) {
    return Xt(this, "push", e);
  },
  reduce(e, ...t) {
    return ni(this, "reduce", e, t);
  },
  reduceRight(e, ...t) {
    return ni(this, "reduceRight", e, t);
  },
  shift() {
    return Xt(this, "shift");
  },
  // slice could use ARRAY_ITERATE but also seems to beg for range tracking
  some(e, t) {
    return rt(this, "some", e, t, void 0, arguments);
  },
  splice(...e) {
    return Xt(this, "splice", e);
  },
  toReversed() {
    return Dt(this).toReversed();
  },
  toSorted(e) {
    return Dt(this).toSorted(e);
  },
  toSpliced(...e) {
    return Dt(this).toSpliced(...e);
  },
  unshift(...e) {
    return Xt(this, "unshift", e);
  },
  values() {
    return os(this, "values", (e) => Ye(this, e));
  }
};
function os(e, t, n) {
  const s = zn(e), i = s[t]();
  return s !== e && !/* @__PURE__ */ Fe(e) && (i._next = i.next, i.next = () => {
    const r = i._next();
    return r.done || (r.value = n(r.value)), r;
  }), i;
}
const To = Array.prototype;
function rt(e, t, n, s, i, r) {
  const o = zn(e), l = o !== e && !/* @__PURE__ */ Fe(e), a = o[t];
  if (a !== To[t]) {
    const h = a.apply(e, r);
    return l ? Ve(h) : h;
  }
  let d = n;
  o !== e && (l ? d = function(h, v) {
    return n.call(this, Ye(e, h), v, e);
  } : n.length > 2 && (d = function(h, v) {
    return n.call(this, h, v, e);
  }));
  const c = a.call(o, d, s);
  return l && i ? i(c) : c;
}
function ni(e, t, n, s) {
  const i = zn(e), r = i !== e && !/* @__PURE__ */ Fe(e);
  let o = n, l = !1;
  i !== e && (r ? (l = s.length === 0, o = function(d, c, h) {
    return l && (l = !1, d = Ye(e, d)), n.call(this, d, Ye(e, c), h, e);
  }) : n.length > 3 && (o = function(d, c, h) {
    return n.call(this, d, c, h, e);
  }));
  const a = i[t](o, ...s);
  return l ? Ye(e, a) : a;
}
function ls(e, t, n) {
  const s = /* @__PURE__ */ re(e);
  xe(s, "iterate", hn);
  const i = s[t](...n);
  return (i === -1 || i === !1) && /* @__PURE__ */ js(n[0]) ? (n[0] = /* @__PURE__ */ re(n[0]), s[t](...n)) : i;
}
function Xt(e, t, n = []) {
  Ze(), Ds();
  const s = (/* @__PURE__ */ re(e))[t].apply(e, n);
  return Fs(), Qe(), s;
}
const Co = /* @__PURE__ */ ks("__proto__,__v_isRef,__isVue"), rr = new Set(
  /* @__PURE__ */ Object.getOwnPropertyNames(Symbol).filter((e) => e !== "arguments" && e !== "caller").map((e) => Symbol[e]).filter(Je)
);
function Eo(e) {
  Je(e) || (e = String(e));
  const t = /* @__PURE__ */ re(this);
  return xe(t, "has", e), t.hasOwnProperty(e);
}
class or {
  constructor(t = !1, n = !1) {
    this._isReadonly = t, this._isShallow = n;
  }
  get(t, n, s) {
    if (n === "__v_skip") return t.__v_skip;
    const i = this._isReadonly, r = this._isShallow;
    if (n === "__v_isReactive")
      return !i;
    if (n === "__v_isReadonly")
      return i;
    if (n === "__v_isShallow")
      return r;
    if (n === "__v_raw")
      return s === (i ? r ? Do : cr : r ? ur : ar).get(t) || // receiver is not the reactive proxy, but has the same prototype
      // this means the receiver is a user proxy of the reactive proxy
      Object.getPrototypeOf(t) === Object.getPrototypeOf(s) ? t : void 0;
    const o = U(t);
    if (!i) {
      let a;
      if (o && (a = xo[n]))
        return a;
      if (n === "hasOwnProperty")
        return Eo;
    }
    const l = Reflect.get(
      t,
      n,
      // if this is a proxy wrapping a ref, return methods using the raw ref
      // as receiver so that we don't have to call `toRaw` on the ref in all
      // its class methods
      /* @__PURE__ */ _e(t) ? t : s
    );
    if ((Je(n) ? rr.has(n) : Co(n)) || (i || xe(t, "get", n), r))
      return l;
    if (/* @__PURE__ */ _e(l)) {
      const a = o && Ps(n) ? l : l.value;
      return i && le(a) ? /* @__PURE__ */ ws(a) : a;
    }
    return le(l) ? i ? /* @__PURE__ */ ws(l) : /* @__PURE__ */ Bs(l) : l;
  }
}
class lr extends or {
  constructor(t = !1) {
    super(!1, t);
  }
  set(t, n, s, i) {
    let r = t[n];
    const o = U(t) && Ps(n);
    if (!this._isShallow) {
      const d = /* @__PURE__ */ dt(r);
      if (!/* @__PURE__ */ Fe(s) && !/* @__PURE__ */ dt(s) && (r = /* @__PURE__ */ re(r), s = /* @__PURE__ */ re(s)), !o && /* @__PURE__ */ _e(r) && !/* @__PURE__ */ _e(s))
        return d || (r.value = s), !0;
    }
    const l = o ? Number(n) < t.length : oe(t, n), a = Reflect.set(
      t,
      n,
      s,
      /* @__PURE__ */ _e(t) ? t : i
    );
    return t === /* @__PURE__ */ re(i) && a && (l ? ze(s, r) && ut(t, "set", n, s) : ut(t, "add", n, s)), a;
  }
  deleteProperty(t, n) {
    const s = oe(t, n);
    t[n];
    const i = Reflect.deleteProperty(t, n);
    return i && s && ut(t, "delete", n, void 0), i;
  }
  has(t, n) {
    const s = Reflect.has(t, n);
    return (!Je(n) || !rr.has(n)) && xe(t, "has", n), s;
  }
  ownKeys(t) {
    return xe(
      t,
      "iterate",
      U(t) ? "length" : Lt
    ), Reflect.ownKeys(t);
  }
}
class So extends or {
  constructor(t = !1) {
    super(!0, t);
  }
  set(t, n) {
    return !0;
  }
  deleteProperty(t, n) {
    return !0;
  }
}
const $o = /* @__PURE__ */ new lr(), Ao = /* @__PURE__ */ new So(), Mo = /* @__PURE__ */ new lr(!0);
const bs = (e) => e, En = (e) => Reflect.getPrototypeOf(e);
function ko(e, t, n) {
  return function(...s) {
    const i = this.__v_raw, r = /* @__PURE__ */ re(i), o = Ht(r), l = e === "entries" || e === Symbol.iterator && o, a = e === "keys" && o, d = i[e](...s), c = n ? bs : t ? jt : Ve;
    return !t && xe(
      r,
      "iterate",
      a ? ys : Lt
    ), me(
      // inheriting all iterator properties
      Object.create(d),
      {
        // iterator protocol
        next() {
          const { value: h, done: v } = d.next();
          return v ? { value: h, done: v } : {
            value: l ? [c(h[0]), c(h[1])] : c(h),
            done: v
          };
        }
      }
    );
  };
}
function Sn(e) {
  return function(...t) {
    return e === "delete" ? !1 : e === "clear" ? void 0 : this;
  };
}
function Lo(e, t) {
  const n = {
    get(i) {
      const r = this.__v_raw, o = /* @__PURE__ */ re(r), l = /* @__PURE__ */ re(i);
      e || (ze(i, l) && xe(o, "get", i), xe(o, "get", l));
      const { has: a } = En(o), d = t ? bs : e ? jt : Ve;
      if (a.call(o, i))
        return d(r.get(i));
      if (a.call(o, l))
        return d(r.get(l));
      r !== o && r.get(i);
    },
    get size() {
      const i = this.__v_raw;
      return !e && xe(/* @__PURE__ */ re(i), "iterate", Lt), i.size;
    },
    has(i) {
      const r = this.__v_raw, o = /* @__PURE__ */ re(r), l = /* @__PURE__ */ re(i);
      return e || (ze(i, l) && xe(o, "has", i), xe(o, "has", l)), i === l ? r.has(i) : r.has(i) || r.has(l);
    },
    forEach(i, r) {
      const o = this, l = o.__v_raw, a = /* @__PURE__ */ re(l), d = t ? bs : e ? jt : Ve;
      return !e && xe(a, "iterate", Lt), l.forEach((c, h) => i.call(r, d(c), d(h), o));
    }
  };
  return me(
    n,
    e ? {
      add: Sn("add"),
      set: Sn("set"),
      delete: Sn("delete"),
      clear: Sn("clear")
    } : {
      add(i) {
        const r = /* @__PURE__ */ re(this), o = En(r), l = /* @__PURE__ */ re(i), a = !t && !/* @__PURE__ */ Fe(i) && !/* @__PURE__ */ dt(i) ? l : i;
        return o.has.call(r, a) || ze(i, a) && o.has.call(r, i) || ze(l, a) && o.has.call(r, l) || (r.add(a), ut(r, "add", a, a)), this;
      },
      set(i, r) {
        !t && !/* @__PURE__ */ Fe(r) && !/* @__PURE__ */ dt(r) && (r = /* @__PURE__ */ re(r));
        const o = /* @__PURE__ */ re(this), { has: l, get: a } = En(o);
        let d = l.call(o, i);
        d || (i = /* @__PURE__ */ re(i), d = l.call(o, i));
        const c = a.call(o, i);
        return o.set(i, r), d ? ze(r, c) && ut(o, "set", i, r) : ut(o, "add", i, r), this;
      },
      delete(i) {
        const r = /* @__PURE__ */ re(this), { has: o, get: l } = En(r);
        let a = o.call(r, i);
        a || (i = /* @__PURE__ */ re(i), a = o.call(r, i)), l && l.call(r, i);
        const d = r.delete(i);
        return a && ut(r, "delete", i, void 0), d;
      },
      clear() {
        const i = /* @__PURE__ */ re(this), r = i.size !== 0, o = i.clear();
        return r && ut(
          i,
          "clear",
          void 0,
          void 0
        ), o;
      }
    }
  ), [
    "keys",
    "values",
    "entries",
    Symbol.iterator
  ].forEach((i) => {
    n[i] = ko(i, e, t);
  }), n;
}
function Hs(e, t) {
  const n = Lo(e, t);
  return (s, i, r) => i === "__v_isReactive" ? !e : i === "__v_isReadonly" ? e : i === "__v_raw" ? s : Reflect.get(
    oe(n, i) && i in s ? n : s,
    i,
    r
  );
}
const Po = {
  get: /* @__PURE__ */ Hs(!1, !1)
}, Io = {
  get: /* @__PURE__ */ Hs(!1, !0)
}, Oo = {
  get: /* @__PURE__ */ Hs(!0, !1)
};
const ar = /* @__PURE__ */ new WeakMap(), ur = /* @__PURE__ */ new WeakMap(), cr = /* @__PURE__ */ new WeakMap(), Do = /* @__PURE__ */ new WeakMap();
function Fo(e) {
  switch (e) {
    case "Object":
    case "Array":
      return 1;
    case "Map":
    case "Set":
    case "WeakMap":
    case "WeakSet":
      return 2;
    default:
      return 0;
  }
}
// @__NO_SIDE_EFFECTS__
function Bs(e) {
  return /* @__PURE__ */ dt(e) ? e : Vs(
    e,
    !1,
    $o,
    Po,
    ar
  );
}
// @__NO_SIDE_EFFECTS__
function Ro(e) {
  return Vs(
    e,
    !1,
    Mo,
    Io,
    ur
  );
}
// @__NO_SIDE_EFFECTS__
function ws(e) {
  return Vs(
    e,
    !0,
    Ao,
    Oo,
    cr
  );
}
function Vs(e, t, n, s, i) {
  if (!le(e) || e.__v_raw && !(t && e.__v_isReactive) || e.__v_skip || !Object.isExtensible(e))
    return e;
  const r = i.get(e);
  if (r)
    return r;
  const o = Fo(oo(e));
  if (o === 0)
    return e;
  const l = new Proxy(
    e,
    o === 2 ? s : n
  );
  return i.set(e, l), l;
}
// @__NO_SIDE_EFFECTS__
function Pt(e) {
  return /* @__PURE__ */ dt(e) ? /* @__PURE__ */ Pt(e.__v_raw) : !!(e && e.__v_isReactive);
}
// @__NO_SIDE_EFFECTS__
function dt(e) {
  return !!(e && e.__v_isReadonly);
}
// @__NO_SIDE_EFFECTS__
function Fe(e) {
  return !!(e && e.__v_isShallow);
}
// @__NO_SIDE_EFFECTS__
function js(e) {
  return e ? !!e.__v_raw : !1;
}
// @__NO_SIDE_EFFECTS__
function re(e) {
  const t = e && e.__v_raw;
  return t ? /* @__PURE__ */ re(t) : e;
}
function No(e) {
  return !oe(e, "__v_skip") && Object.isExtensible(e) && Yi(e, "__v_skip", !0), e;
}
const Ve = (e) => le(e) ? /* @__PURE__ */ Bs(e) : e, jt = (e) => le(e) ? /* @__PURE__ */ ws(e) : e;
// @__NO_SIDE_EFFECTS__
function _e(e) {
  return e ? e.__v_isRef === !0 : !1;
}
// @__NO_SIDE_EFFECTS__
function z(e) {
  return Ho(e, !1);
}
function Ho(e, t) {
  return /* @__PURE__ */ _e(e) ? e : new Bo(e, t);
}
class Bo {
  constructor(t, n) {
    this.dep = new Ns(), this.__v_isRef = !0, this.__v_isShallow = !1, this._rawValue = n ? t : /* @__PURE__ */ re(t), this._value = n ? t : Ve(t), this.__v_isShallow = n;
  }
  get value() {
    return this.dep.track(), this._value;
  }
  set value(t) {
    const n = this._rawValue, s = this.__v_isShallow || /* @__PURE__ */ Fe(t) || /* @__PURE__ */ dt(t);
    t = s ? t : /* @__PURE__ */ re(t), ze(t, n) && (this._rawValue = t, this._value = s ? t : Ve(t), this.dep.trigger());
  }
}
function we(e) {
  return /* @__PURE__ */ _e(e) ? e.value : e;
}
const Vo = {
  get: (e, t, n) => t === "__v_raw" ? e : we(Reflect.get(e, t, n)),
  set: (e, t, n, s) => {
    const i = e[t];
    return /* @__PURE__ */ _e(i) && !/* @__PURE__ */ _e(n) ? (i.value = n, !0) : Reflect.set(e, t, n, s);
  }
};
function fr(e) {
  return /* @__PURE__ */ Pt(e) ? e : new Proxy(e, Vo);
}
class jo {
  constructor(t, n, s) {
    this.fn = t, this.setter = n, this._value = void 0, this.dep = new Ns(this), this.__v_isRef = !0, this.deps = void 0, this.depsTail = void 0, this.flags = 16, this.globalVersion = dn - 1, this.next = void 0, this.effect = this, this.__v_isReadonly = !n, this.isSSR = s;
  }
  /**
   * @internal
   */
  notify() {
    if (this.flags |= 16, !(this.flags & 8) && // avoid infinite self recursion
    fe !== this)
      return Qi(this, !0), !0;
  }
  get value() {
    const t = this.dep.track();
    return nr(this), t && (t.version = this.dep.version), this._value;
  }
  set value(t) {
    this.setter && this.setter(t);
  }
}
// @__NO_SIDE_EFFECTS__
function Ko(e, t, n = !1) {
  let s, i;
  return X(e) ? s = e : (s = e.get, i = e.set), new jo(s, i, n);
}
const $n = {}, On = /* @__PURE__ */ new WeakMap();
let At;
function Uo(e, t = !1, n = At) {
  if (n) {
    let s = On.get(n);
    s || On.set(n, s = []), s.push(e);
  }
}
function Wo(e, t, n = ce) {
  const { immediate: s, deep: i, once: r, scheduler: o, augmentJob: l, call: a } = n, d = (T) => i ? T : /* @__PURE__ */ Fe(T) || i === !1 || i === 0 ? ct(T, 1) : ct(T);
  let c, h, v, _, C = !1, M = !1;
  if (/* @__PURE__ */ _e(e) ? (h = () => e.value, C = /* @__PURE__ */ Fe(e)) : /* @__PURE__ */ Pt(e) ? (h = () => d(e), C = !0) : U(e) ? (M = !0, C = e.some((T) => /* @__PURE__ */ Pt(T) || /* @__PURE__ */ Fe(T)), h = () => e.map((T) => {
    if (/* @__PURE__ */ _e(T))
      return T.value;
    if (/* @__PURE__ */ Pt(T))
      return d(T);
    if (X(T))
      return a ? a(T, 2) : T();
  })) : X(e) ? t ? h = a ? () => a(e, 2) : e : h = () => {
    if (v) {
      Ze();
      try {
        v();
      } finally {
        Qe();
      }
    }
    const T = At;
    At = c;
    try {
      return a ? a(e, 3, [_]) : e(_);
    } finally {
      At = T;
    }
  } : h = Ge, t && i) {
    const T = h, K = i === !0 ? 1 / 0 : i;
    h = () => ct(T(), K);
  }
  const F = bo(), D = () => {
    c.stop(), F && F.active && Ls(F.effects, c);
  };
  if (r && t) {
    const T = t;
    t = (...K) => {
      const R = T(...K);
      return D(), R;
    };
  }
  let L = M ? new Array(e.length).fill($n) : $n;
  const N = (T) => {
    if (!(!(c.flags & 1) || !c.dirty && !T))
      if (t) {
        const K = c.run();
        if (T || i || C || (M ? K.some((R, E) => ze(R, L[E])) : ze(K, L))) {
          v && v();
          const R = At;
          At = c;
          try {
            const E = [
              K,
              // pass undefined as the old value when it's changed for the first time
              L === $n ? void 0 : M && L[0] === $n ? [] : L,
              _
            ];
            L = K, a ? a(t, 3, E) : (
              // @ts-expect-error
              t(...E)
            );
          } finally {
            At = R;
          }
        }
      } else
        c.run();
  };
  return l && l(N), c = new Ji(h), c.scheduler = o ? () => o(N, !1) : N, _ = (T) => Uo(T, !1, c), v = c.onStop = () => {
    const T = On.get(c);
    if (T) {
      if (a)
        a(T, 4);
      else
        for (const K of T) K();
      On.delete(c);
    }
  }, t ? s ? N(!0) : L = c.run() : o ? o(N.bind(null, !0), !0) : c.run(), D.pause = c.pause.bind(c), D.resume = c.resume.bind(c), D.stop = D, D;
}
function ct(e, t = 1 / 0, n) {
  if (t <= 0 || !le(e) || e.__v_skip || (n = n || /* @__PURE__ */ new Map(), (n.get(e) || 0) >= t))
    return e;
  if (n.set(e, t), t--, /* @__PURE__ */ _e(e))
    ct(e.value, t, n);
  else if (U(e))
    for (let s = 0; s < e.length; s++)
      ct(e[s], t, n);
  else if (ji(e) || Ht(e))
    e.forEach((s) => {
      ct(s, t, n);
    });
  else if (Wi(e)) {
    for (const s in e)
      ct(e[s], t, n);
    for (const s of Object.getOwnPropertySymbols(e))
      Object.prototype.propertyIsEnumerable.call(e, s) && ct(e[s], t, n);
  }
  return e;
}
function wn(e, t, n, s) {
  try {
    return s ? e(...s) : e();
  } catch (i) {
    Gn(i, t, n);
  }
}
function Re(e, t, n, s) {
  if (X(e)) {
    const i = wn(e, t, n, s);
    return i && Ki(i) && i.catch((r) => {
      Gn(r, t, n);
    }), i;
  }
  if (U(e)) {
    const i = [];
    for (let r = 0; r < e.length; r++)
      i.push(Re(e[r], t, n, s));
    return i;
  }
}
function Gn(e, t, n, s = !0) {
  const i = t ? t.vnode : null, { errorHandler: r, throwUnhandledErrorInProduction: o } = t && t.appContext.config || ce;
  if (t) {
    let l = t.parent;
    const a = t.proxy, d = `https://vuejs.org/error-reference/#runtime-${n}`;
    for (; l; ) {
      const c = l.ec;
      if (c) {
        for (let h = 0; h < c.length; h++)
          if (c[h](e, a, d) === !1)
            return;
      }
      l = l.parent;
    }
    if (r) {
      Ze(), wn(r, null, 10, [
        e,
        a,
        d
      ]), Qe();
      return;
    }
  }
  qo(e, n, i, s, o);
}
function qo(e, t, n, s = !0, i = !1) {
  if (i)
    throw e;
  console.error(e);
}
const Ee = [];
let qe = -1;
const Bt = [];
let bt = null, Ft = 0;
const dr = /* @__PURE__ */ Promise.resolve();
let Dn = null;
function ht(e) {
  const t = Dn || dr;
  return e ? t.then(this ? e.bind(this) : e) : t;
}
function Yo(e) {
  let t = qe + 1, n = Ee.length;
  for (; t < n; ) {
    const s = t + n >>> 1, i = Ee[s], r = pn(i);
    r < e || r === e && i.flags & 2 ? t = s + 1 : n = s;
  }
  return t;
}
function Ks(e) {
  if (!(e.flags & 1)) {
    const t = pn(e), n = Ee[Ee.length - 1];
    !n || // fast path when the job id is larger than the tail
    !(e.flags & 2) && t >= pn(n) ? Ee.push(e) : Ee.splice(Yo(t), 0, e), e.flags |= 1, hr();
  }
}
function hr() {
  Dn || (Dn = dr.then(gr));
}
function Xo(e) {
  U(e) ? Bt.push(...e) : bt && e.id === -1 ? bt.splice(Ft + 1, 0, e) : e.flags & 1 || (Bt.push(e), e.flags |= 1), hr();
}
function si(e, t, n = qe + 1) {
  for (; n < Ee.length; n++) {
    const s = Ee[n];
    if (s && s.flags & 2) {
      if (e && s.id !== e.uid)
        continue;
      Ee.splice(n, 1), n--, s.flags & 4 && (s.flags &= -2), s(), s.flags & 4 || (s.flags &= -2);
    }
  }
}
function pr(e) {
  if (Bt.length) {
    const t = [...new Set(Bt)].sort(
      (n, s) => pn(n) - pn(s)
    );
    if (Bt.length = 0, bt) {
      bt.push(...t);
      return;
    }
    for (bt = t, Ft = 0; Ft < bt.length; Ft++) {
      const n = bt[Ft];
      n.flags & 4 && (n.flags &= -2), n.flags & 8 || n(), n.flags &= -2;
    }
    bt = null, Ft = 0;
  }
}
const pn = (e) => e.id == null ? e.flags & 2 ? -1 : 1 / 0 : e.id;
function gr(e) {
  try {
    for (qe = 0; qe < Ee.length; qe++) {
      const t = Ee[qe];
      t && !(t.flags & 8) && (t.flags & 4 && (t.flags &= -2), wn(
        t,
        t.i,
        t.i ? 15 : 14
      ), t.flags & 4 || (t.flags &= -2));
    }
  } finally {
    for (; qe < Ee.length; qe++) {
      const t = Ee[qe];
      t && (t.flags &= -2);
    }
    qe = -1, Ee.length = 0, pr(), Dn = null, (Ee.length || Bt.length) && gr();
  }
}
let De = null, mr = null;
function Fn(e) {
  const t = De;
  return De = e, mr = e && e.type.__scopeId || null, t;
}
function Ut(e, t = De, n) {
  if (!t || e._n)
    return e;
  const s = (...i) => {
    s._d && Hn(-1);
    const r = Fn(t);
    let o;
    try {
      o = e(...i);
    } finally {
      Fn(r), s._d && Hn(1);
    }
    return o;
  };
  return s._n = !0, s._c = !0, s._d = !0, s;
}
function vr(e, t) {
  if (De === null)
    return e;
  const n = ts(De), s = e.dirs || (e.dirs = []);
  for (let i = 0; i < t.length; i++) {
    let [r, o, l, a = ce] = t[i];
    r && (X(r) && (r = {
      mounted: r,
      updated: r
    }), r.deep && ct(o), s.push({
      dir: r,
      instance: n,
      value: o,
      oldValue: void 0,
      arg: l,
      modifiers: a
    }));
  }
  return e;
}
function Ct(e, t, n, s) {
  const i = e.dirs, r = t && t.dirs;
  for (let o = 0; o < i.length; o++) {
    const l = i[o];
    r && (l.oldValue = r[o].value);
    let a = l.dir[s];
    a && (Ze(), Re(a, n, 8, [
      e.el,
      l,
      e,
      t
    ]), Qe());
  }
}
function zo(e, t) {
  if ($e) {
    let n = $e.provides;
    const s = $e.parent && $e.parent.provides;
    s === n && (n = $e.provides = Object.create(s)), n[e] = t;
  }
}
function Ln(e, t, n = !1) {
  const s = zr();
  if (s || Vt) {
    let i = Vt ? Vt._context.provides : s ? s.parent == null || s.ce ? s.vnode.appContext && s.vnode.appContext.provides : s.parent.provides : void 0;
    if (i && e in i)
      return i[e];
    if (arguments.length > 1)
      return n && X(t) ? t.call(s && s.proxy) : t;
  }
}
const Go = /* @__PURE__ */ Symbol.for("v-scx"), Jo = () => Ln(Go);
function ln(e, t, n) {
  return yr(e, t, n);
}
function yr(e, t, n = ce) {
  const { immediate: s, deep: i, flush: r, once: o } = n, l = me({}, n), a = t && s || !t && r !== "post";
  let d;
  if (vn) {
    if (r === "sync") {
      const _ = Jo();
      d = _.__watcherHandles || (_.__watcherHandles = []);
    } else if (!a) {
      const _ = () => {
      };
      return _.stop = Ge, _.resume = Ge, _.pause = Ge, _;
    }
  }
  const c = $e;
  l.call = (_, C, M) => Re(_, c, C, M);
  let h = !1;
  r === "post" ? l.scheduler = (_) => {
    Ce(_, c && c.suspense);
  } : r !== "sync" && (h = !0, l.scheduler = (_, C) => {
    C ? _() : Ks(_);
  }), l.augmentJob = (_) => {
    t && (_.flags |= 4), h && (_.flags |= 2, c && (_.id = c.uid, _.i = c));
  };
  const v = Wo(e, t, l);
  return vn && (d ? d.push(v) : a && v()), v;
}
function Zo(e, t, n) {
  const s = this.proxy, i = he(e) ? e.includes(".") ? br(s, e) : () => s[e] : e.bind(s, s);
  let r;
  X(t) ? r = t : (r = t.handler, n = t);
  const o = _n(this), l = yr(i, r.bind(s), n);
  return o(), l;
}
function br(e, t) {
  const n = t.split(".");
  return () => {
    let s = e;
    for (let i = 0; i < n.length && s; i++)
      s = s[n[i]];
    return s;
  };
}
const yt = /* @__PURE__ */ new WeakMap(), wr = /* @__PURE__ */ Symbol("_vte"), _r = (e) => e.__isTeleport, Mt = (e) => e && (e.disabled || e.disabled === ""), Qo = (e) => e && (e.defer || e.defer === ""), ii = (e) => typeof SVGElement < "u" && e instanceof SVGElement, ri = (e) => typeof MathMLElement == "function" && e instanceof MathMLElement, _s = (e, t) => {
  const n = e && e.to;
  return he(n) ? t ? t(n) : null : n;
}, el = {
  name: "Teleport",
  __isTeleport: !0,
  process(e, t, n, s, i, r, o, l, a, d) {
    const {
      mc: c,
      pc: h,
      pbc: v,
      o: { insert: _, querySelector: C, createText: M, createComment: F, parentNode: D }
    } = d, L = Mt(t.props);
    let { dynamicChildren: N } = t;
    const T = (E, H, x) => {
      E.shapeFlag & 16 && c(
        E.children,
        H,
        x,
        i,
        r,
        o,
        l,
        a
      );
    }, K = (E = t) => {
      const H = Mt(E.props), x = E.target = _s(E.props, C), B = xs(x, E, M, _);
      x && (o !== "svg" && ii(x) ? o = "svg" : o !== "mathml" && ri(x) && (o = "mathml"), i && i.isCE && (i.ce._teleportTargets || (i.ce._teleportTargets = /* @__PURE__ */ new Set())).add(x), H || (T(E, x, B), Zt(E, !1)));
    }, R = (E) => {
      const H = () => {
        if (yt.get(E) === H) {
          if (yt.delete(E), Mt(E.props)) {
            const x = D(E.el) || n;
            T(E, x, E.anchor), Zt(E, !0);
          }
          K(E);
        }
      };
      yt.set(E, H), Ce(H, r);
    };
    if (e == null) {
      const E = t.el = M(""), H = t.anchor = M("");
      if (_(E, n, s), _(H, n, s), Qo(t.props) || r && r.pendingBranch) {
        R(t);
        return;
      }
      L && (T(t, n, H), Zt(t, !0)), K();
    } else {
      t.el = e.el;
      const E = t.anchor = e.anchor, H = yt.get(e);
      if (H) {
        H.flags |= 8, yt.delete(e), R(t);
        return;
      }
      t.targetStart = e.targetStart;
      const x = t.target = e.target, B = t.targetAnchor = e.targetAnchor, G = Mt(e.props), P = G ? n : x, ie = G ? E : B;
      if (o === "svg" || ii(x) ? o = "svg" : (o === "mathml" || ri(x)) && (o = "mathml"), N ? (v(
        e.dynamicChildren,
        N,
        P,
        i,
        r,
        o,
        l
      ), qs(e, t, !0)) : a || h(
        e,
        t,
        P,
        ie,
        i,
        r,
        o,
        l,
        !1
      ), L)
        G ? t.props && e.props && t.props.to !== e.props.to && (t.props.to = e.props.to) : An(
          t,
          n,
          E,
          d,
          1
        );
      else if ((t.props && t.props.to) !== (e.props && e.props.to)) {
        const W = _s(t.props, C);
        W && (t.target = W, An(
          t,
          W,
          null,
          d,
          0
        ));
      } else G && An(
        t,
        x,
        B,
        d,
        1
      );
      Zt(t, L);
    }
  },
  remove(e, t, n, { um: s, o: { remove: i } }, r) {
    const {
      shapeFlag: o,
      children: l,
      anchor: a,
      targetStart: d,
      targetAnchor: c,
      target: h,
      props: v
    } = e, _ = Mt(v), C = r || !_, M = yt.get(e);
    if (M && (M.flags |= 8, yt.delete(e)), h && (i(d), i(c)), r && i(a), !M && (_ || h) && o & 16)
      for (let F = 0; F < l.length; F++) {
        const D = l[F];
        s(
          D,
          t,
          n,
          C,
          !!D.dynamicChildren
        );
      }
  },
  move: An,
  hydrate: tl
};
function An(e, t, n, { o: { insert: s }, m: i }, r = 2) {
  r === 0 && s(e.targetAnchor, t, n);
  const { el: o, anchor: l, shapeFlag: a, children: d, props: c } = e, h = r === 2;
  if (h && s(o, t, n), !yt.has(e) && (!h || Mt(c)) && a & 16)
    for (let v = 0; v < d.length; v++)
      i(
        d[v],
        t,
        n,
        2
      );
  h && s(l, t, n);
}
function tl(e, t, n, s, i, r, {
  o: { nextSibling: o, parentNode: l, querySelector: a, insert: d, createText: c }
}, h) {
  function v(F, D) {
    let L = D;
    for (; L; ) {
      if (L && L.nodeType === 8) {
        if (L.data === "teleport start anchor")
          t.targetStart = L;
        else if (L.data === "teleport anchor") {
          t.targetAnchor = L, F._lpa = t.targetAnchor && o(t.targetAnchor);
          break;
        }
      }
      L = o(L);
    }
  }
  function _(F, D) {
    D.anchor = h(
      o(F),
      D,
      l(F),
      n,
      s,
      i,
      r
    );
  }
  const C = t.target = _s(
    t.props,
    a
  ), M = Mt(t.props);
  if (C) {
    const F = C._lpa || C.firstChild;
    t.shapeFlag & 16 && (M ? (_(e, t), v(C, F), t.targetAnchor || xs(
      C,
      t,
      c,
      d,
      // if target is the same as the main view, insert anchors before current node
      // to avoid hydrating mismatch
      l(e) === C ? e : null
    )) : (t.anchor = o(e), v(C, F), t.targetAnchor || xs(C, t, c, d), h(
      F && o(F),
      t,
      C,
      n,
      s,
      i,
      r
    ))), Zt(t, M);
  } else M && t.shapeFlag & 16 && (_(e, t), t.targetStart = e, t.targetAnchor = o(e));
  return t.anchor && o(t.anchor);
}
const xr = el;
function Zt(e, t) {
  const n = e.ctx;
  if (n && n.ut) {
    let s, i;
    for (t ? (s = e.el, i = e.anchor) : (s = e.targetStart, i = e.targetAnchor); s && s !== i; )
      s.nodeType === 1 && s.setAttribute("data-v-owner", n.uid), s = s.nextSibling;
    n.ut();
  }
}
function xs(e, t, n, s, i = null) {
  const r = t.targetStart = n(""), o = t.targetAnchor = n("");
  return r[wr] = o, e && (s(r, e, i), s(o, e, i)), o;
}
const Oe = /* @__PURE__ */ Symbol("_leaveCb"), zt = /* @__PURE__ */ Symbol("_enterCb");
function nl() {
  const e = {
    isMounted: !1,
    isLeaving: !1,
    isUnmounting: !1,
    leavingVNodes: /* @__PURE__ */ new Map()
  };
  return gt(() => {
    e.isMounted = !0;
  }), xt(() => {
    e.isUnmounting = !0;
  }), e;
}
const Ie = [Function, Array], Tr = {
  mode: String,
  appear: Boolean,
  persisted: Boolean,
  // enter
  onBeforeEnter: Ie,
  onEnter: Ie,
  onAfterEnter: Ie,
  onEnterCancelled: Ie,
  // leave
  onBeforeLeave: Ie,
  onLeave: Ie,
  onAfterLeave: Ie,
  onLeaveCancelled: Ie,
  // appear
  onBeforeAppear: Ie,
  onAppear: Ie,
  onAfterAppear: Ie,
  onAppearCancelled: Ie
}, Cr = (e) => {
  const t = e.subTree;
  return t.component ? Cr(t.component) : t;
}, sl = {
  name: "BaseTransition",
  props: Tr,
  setup(e, { slots: t }) {
    const n = zr(), s = nl();
    return () => {
      const i = t.default && $r(t.default(), !0), r = i && i.length ? Er(i) : (
        // Keep explicit default-slot conditionals on the same transition path
        // as regular v-if branches, which render a comment placeholder.
        n.subTree ? de() : void 0
      );
      if (!r)
        return;
      const o = /* @__PURE__ */ re(e), { mode: l } = o;
      if (s.isLeaving)
        return as(r);
      const a = oi(r);
      if (!a)
        return as(r);
      let d = Ts(
        a,
        o,
        s,
        n,
        // #11061, ensure enterHooks is fresh after clone
        (h) => d = h
      );
      a.type !== Se && gn(a, d);
      let c = n.subTree && oi(n.subTree);
      if (c && c.type !== Se && !kt(c, a) && Cr(n).type !== Se) {
        let h = Ts(
          c,
          o,
          s,
          n
        );
        if (gn(c, h), l === "out-in" && a.type !== Se)
          return s.isLeaving = !0, h.afterLeave = () => {
            s.isLeaving = !1, n.job.flags & 8 || n.update(), delete h.afterLeave, c = void 0;
          }, as(r);
        l === "in-out" && a.type !== Se ? h.delayLeave = (v, _, C) => {
          const M = Sr(
            s,
            c
          );
          M[String(c.key)] = c, v[Oe] = () => {
            _(), v[Oe] = void 0, delete d.delayedLeave, c = void 0;
          }, d.delayedLeave = () => {
            C(), delete d.delayedLeave, c = void 0;
          };
        } : c = void 0;
      } else c && (c = void 0);
      return r;
    };
  }
};
function Er(e) {
  let t = e[0];
  if (e.length > 1) {
    for (const n of e)
      if (n.type !== Se) {
        t = n;
        break;
      }
  }
  return t;
}
const il = sl;
function Sr(e, t) {
  const { leavingVNodes: n } = e;
  let s = n.get(t.type);
  return s || (s = /* @__PURE__ */ Object.create(null), n.set(t.type, s)), s;
}
function Ts(e, t, n, s, i) {
  const {
    appear: r,
    mode: o,
    persisted: l = !1,
    onBeforeEnter: a,
    onEnter: d,
    onAfterEnter: c,
    onEnterCancelled: h,
    onBeforeLeave: v,
    onLeave: _,
    onAfterLeave: C,
    onLeaveCancelled: M,
    onBeforeAppear: F,
    onAppear: D,
    onAfterAppear: L,
    onAppearCancelled: N
  } = t, T = String(e.key), K = Sr(n, e), R = (x, B) => {
    x && Re(
      x,
      s,
      9,
      B
    );
  }, E = (x, B) => {
    const G = B[1];
    R(x, B), U(x) ? x.every((P) => P.length <= 1) && G() : x.length <= 1 && G();
  }, H = {
    mode: o,
    persisted: l,
    beforeEnter(x) {
      let B = a;
      if (!n.isMounted)
        if (r)
          B = F || a;
        else
          return;
      x[Oe] && x[Oe](
        !0
        /* cancelled */
      );
      const G = K[T];
      G && kt(e, G) && G.el[Oe] && G.el[Oe](), R(B, [x]);
    },
    enter(x) {
      if (K[T] === e) return;
      let B = d, G = c, P = h;
      if (!n.isMounted)
        if (r)
          B = D || d, G = L || c, P = N || h;
        else
          return;
      let ie = !1;
      x[zt] = (J) => {
        ie || (ie = !0, J ? R(P, [x]) : R(G, [x]), H.delayedLeave && H.delayedLeave(), x[zt] = void 0);
      };
      const W = x[zt].bind(null, !1);
      B ? E(B, [x, W]) : W();
    },
    leave(x, B) {
      const G = String(e.key);
      if (x[zt] && x[zt](
        !0
        /* cancelled */
      ), n.isUnmounting)
        return B();
      R(v, [x]);
      let P = !1;
      x[Oe] = (W) => {
        P || (P = !0, B(), W ? R(M, [x]) : R(C, [x]), x[Oe] = void 0, K[G] === e && delete K[G]);
      };
      const ie = x[Oe].bind(null, !1);
      K[G] = e, _ ? E(_, [x, ie]) : ie();
    },
    clone(x) {
      const B = Ts(
        x,
        t,
        n,
        s,
        i
      );
      return i && i(B), B;
    }
  };
  return H;
}
function as(e) {
  if (Jn(e))
    return e = _t(e), e.children = null, e;
}
function oi(e) {
  if (!Jn(e))
    return _r(e.type) && e.children ? Er(e.children) : e;
  if (e.component)
    return e.component.subTree;
  const { shapeFlag: t, children: n } = e;
  if (n) {
    if (t & 16)
      return n[0];
    if (t & 32 && X(n.default))
      return n.default();
  }
}
function gn(e, t) {
  e.shapeFlag & 6 && e.component ? (e.transition = t, gn(e.component.subTree, t)) : e.shapeFlag & 128 ? (e.ssContent.transition = t.clone(e.ssContent), e.ssFallback.transition = t.clone(e.ssFallback)) : e.transition = t;
}
function $r(e, t = !1, n) {
  let s = [], i = 0;
  for (let r = 0; r < e.length; r++) {
    let o = e[r];
    const l = n == null ? o.key : String(n) + String(o.key != null ? o.key : r);
    o.type === te ? (o.patchFlag & 128 && i++, s = s.concat(
      $r(o.children, t, l)
    )) : (t || o.type !== Se) && s.push(l != null ? _t(o, { key: l }) : o);
  }
  if (i > 1)
    for (let r = 0; r < s.length; r++)
      s[r].patchFlag = -2;
  return s;
}
// @__NO_SIDE_EFFECTS__
function tt(e, t) {
  return X(e) ? (
    // #8236: extend call and options.name access are considered side-effects
    // by Rollup, so we have to wrap it in a pure-annotated IIFE.
    me({ name: e.name }, t, { setup: e })
  ) : e;
}
function Ar(e) {
  e.ids = [e.ids[0] + e.ids[2]++ + "-", 0, 0];
}
function li(e, t) {
  let n;
  return !!((n = Object.getOwnPropertyDescriptor(e, t)) && !n.configurable);
}
const Rn = /* @__PURE__ */ new WeakMap();
function an(e, t, n, s, i = !1) {
  if (U(e)) {
    e.forEach(
      (M, F) => an(
        M,
        t && (U(t) ? t[F] : t),
        n,
        s,
        i
      )
    );
    return;
  }
  if (un(s) && !i) {
    s.shapeFlag & 512 && s.type.__asyncResolved && s.component.subTree.component && an(e, t, n, s.component.subTree);
    return;
  }
  const r = s.shapeFlag & 4 ? ts(s.component) : s.el, o = i ? null : r, { i: l, r: a } = e, d = t && t.r, c = l.refs === ce ? l.refs = {} : l.refs, h = l.setupState, v = /* @__PURE__ */ re(h), _ = h === ce ? Vi : (M) => li(c, M) ? !1 : oe(v, M), C = (M, F) => !(F && li(c, F));
  if (d != null && d !== a) {
    if (ai(t), he(d))
      c[d] = null, _(d) && (h[d] = null);
    else if (/* @__PURE__ */ _e(d)) {
      const M = t;
      C(d, M.k) && (d.value = null), M.k && (c[M.k] = null);
    }
  }
  if (X(a)) {
    Ze();
    try {
      wn(a, l, 12, [o, c]);
    } finally {
      Qe();
    }
  } else {
    const M = he(a), F = /* @__PURE__ */ _e(a);
    if (M || F) {
      const D = () => {
        if (e.f) {
          const L = M ? _(a) ? h[a] : c[a] : C() || !e.k ? a.value : c[e.k];
          if (i)
            U(L) && Ls(L, r);
          else if (U(L))
            L.includes(r) || L.push(r);
          else if (M)
            c[a] = [r], _(a) && (h[a] = c[a]);
          else {
            const N = [r];
            C(a, e.k) && (a.value = N), e.k && (c[e.k] = N);
          }
        } else M ? (c[a] = o, _(a) && (h[a] = o)) : F && (C(a, e.k) && (a.value = o), e.k && (c[e.k] = o));
      };
      if (o) {
        const L = () => {
          D(), Rn.delete(e);
        };
        L.id = -1, Rn.set(e, L), Ce(L, n);
      } else
        ai(e), D();
    }
  }
}
function ai(e) {
  const t = Rn.get(e);
  t && (t.flags |= 8, Rn.delete(e));
}
Xn().requestIdleCallback;
Xn().cancelIdleCallback;
const un = (e) => !!e.type.__asyncLoader, Jn = (e) => e.type.__isKeepAlive;
function rl(e, t) {
  Mr(e, "a", t);
}
function ol(e, t) {
  Mr(e, "da", t);
}
function Mr(e, t, n = $e) {
  const s = e.__wdc || (e.__wdc = () => {
    let i = n;
    for (; i; ) {
      if (i.isDeactivated)
        return;
      i = i.parent;
    }
    return e();
  });
  if (Zn(t, s, n), n) {
    let i = n.parent;
    for (; i && i.parent; )
      Jn(i.parent.vnode) && ll(s, t, n, i), i = i.parent;
  }
}
function ll(e, t, n, s) {
  const i = Zn(
    t,
    e,
    s,
    !0
    /* prepend */
  );
  kr(() => {
    Ls(s[t], i);
  }, n);
}
function Zn(e, t, n = $e, s = !1) {
  if (n) {
    const i = n[e] || (n[e] = []), r = t.__weh || (t.__weh = (...o) => {
      Ze();
      const l = _n(n), a = Re(t, n, e, o);
      return l(), Qe(), a;
    });
    return s ? i.unshift(r) : i.push(r), r;
  }
}
const pt = (e) => (t, n = $e) => {
  (!vn || e === "sp") && Zn(e, (...s) => t(...s), n);
}, al = pt("bm"), gt = pt("m"), ul = pt(
  "bu"
), cl = pt("u"), xt = pt(
  "bum"
), kr = pt("um"), fl = pt(
  "sp"
), dl = pt("rtg"), hl = pt("rtc");
function pl(e, t = $e) {
  Zn("ec", e, t);
}
const gl = /* @__PURE__ */ Symbol.for("v-ndc");
function ft(e, t, n, s) {
  let i;
  const r = n, o = U(e);
  if (o || he(e)) {
    const l = o && /* @__PURE__ */ Pt(e);
    let a = !1, d = !1;
    l && (a = !/* @__PURE__ */ Fe(e), d = /* @__PURE__ */ dt(e), e = zn(e)), i = new Array(e.length);
    for (let c = 0, h = e.length; c < h; c++)
      i[c] = t(
        a ? d ? jt(Ve(e[c])) : Ve(e[c]) : e[c],
        c,
        void 0,
        r
      );
  } else if (typeof e == "number") {
    i = new Array(e);
    for (let l = 0; l < e; l++)
      i[l] = t(l + 1, l, void 0, r);
  } else if (le(e))
    if (e[Symbol.iterator])
      i = Array.from(
        e,
        (l, a) => t(l, a, void 0, r)
      );
    else {
      const l = Object.keys(e);
      i = new Array(l.length);
      for (let a = 0, d = l.length; a < d; a++) {
        const c = l[a];
        i[a] = t(e[c], c, a, r);
      }
    }
  else
    i = [];
  return i;
}
const Cs = (e) => e ? Gr(e) ? ts(e) : Cs(e.parent) : null, cn = (
  // Move PURE marker to new line to workaround compiler discarding it
  // due to type annotation
  /* @__PURE__ */ me(/* @__PURE__ */ Object.create(null), {
    $: (e) => e,
    $el: (e) => e.vnode.el,
    $data: (e) => e.data,
    $props: (e) => e.props,
    $attrs: (e) => e.attrs,
    $slots: (e) => e.slots,
    $refs: (e) => e.refs,
    $parent: (e) => Cs(e.parent),
    $root: (e) => Cs(e.root),
    $host: (e) => e.ce,
    $emit: (e) => e.emit,
    $options: (e) => Pr(e),
    $forceUpdate: (e) => e.f || (e.f = () => {
      Ks(e.update);
    }),
    $nextTick: (e) => e.n || (e.n = ht.bind(e.proxy)),
    $watch: (e) => Zo.bind(e)
  })
), us = (e, t) => e !== ce && !e.__isScriptSetup && oe(e, t), ml = {
  get({ _: e }, t) {
    if (t === "__v_skip")
      return !0;
    const { ctx: n, setupState: s, data: i, props: r, accessCache: o, type: l, appContext: a } = e;
    if (t[0] !== "$") {
      const v = o[t];
      if (v !== void 0)
        switch (v) {
          case 1:
            return s[t];
          case 2:
            return i[t];
          case 4:
            return n[t];
          case 3:
            return r[t];
        }
      else {
        if (us(s, t))
          return o[t] = 1, s[t];
        if (i !== ce && oe(i, t))
          return o[t] = 2, i[t];
        if (oe(r, t))
          return o[t] = 3, r[t];
        if (n !== ce && oe(n, t))
          return o[t] = 4, n[t];
        Es && (o[t] = 0);
      }
    }
    const d = cn[t];
    let c, h;
    if (d)
      return t === "$attrs" && xe(e.attrs, "get", ""), d(e);
    if (
      // css module (injected by vue-loader)
      (c = l.__cssModules) && (c = c[t])
    )
      return c;
    if (n !== ce && oe(n, t))
      return o[t] = 4, n[t];
    if (
      // global properties
      h = a.config.globalProperties, oe(h, t)
    )
      return h[t];
  },
  set({ _: e }, t, n) {
    const { data: s, setupState: i, ctx: r } = e;
    return us(i, t) ? (i[t] = n, !0) : s !== ce && oe(s, t) ? (s[t] = n, !0) : oe(e.props, t) || t[0] === "$" && t.slice(1) in e ? !1 : (r[t] = n, !0);
  },
  has({
    _: { data: e, setupState: t, accessCache: n, ctx: s, appContext: i, props: r, type: o }
  }, l) {
    let a;
    return !!(n[l] || e !== ce && l[0] !== "$" && oe(e, l) || us(t, l) || oe(r, l) || oe(s, l) || oe(cn, l) || oe(i.config.globalProperties, l) || (a = o.__cssModules) && a[l]);
  },
  defineProperty(e, t, n) {
    return n.get != null ? e._.accessCache[t] = 0 : oe(n, "value") && this.set(e, t, n.value, null), Reflect.defineProperty(e, t, n);
  }
};
function ui(e) {
  return U(e) ? e.reduce(
    (t, n) => (t[n] = null, t),
    {}
  ) : e;
}
let Es = !0;
function vl(e) {
  const t = Pr(e), n = e.proxy, s = e.ctx;
  Es = !1, t.beforeCreate && ci(t.beforeCreate, e, "bc");
  const {
    // state
    data: i,
    computed: r,
    methods: o,
    watch: l,
    provide: a,
    inject: d,
    // lifecycle
    created: c,
    beforeMount: h,
    mounted: v,
    beforeUpdate: _,
    updated: C,
    activated: M,
    deactivated: F,
    beforeDestroy: D,
    beforeUnmount: L,
    destroyed: N,
    unmounted: T,
    render: K,
    renderTracked: R,
    renderTriggered: E,
    errorCaptured: H,
    serverPrefetch: x,
    // public API
    expose: B,
    inheritAttrs: G,
    // assets
    components: P,
    directives: ie,
    filters: W
  } = t;
  if (d && yl(d, s, null), o)
    for (const Y in o) {
      const Q = o[Y];
      X(Q) && (s[Y] = Q.bind(n));
    }
  if (i) {
    const Y = i.call(n, n);
    le(Y) && (e.data = /* @__PURE__ */ Bs(Y));
  }
  if (Es = !0, r)
    for (const Y in r) {
      const Q = r[Y], Ne = X(Q) ? Q.bind(n, n) : X(Q.get) ? Q.get.bind(n, n) : Ge, Ot = !X(Q) && X(Q.set) ? Q.set.bind(n) : Ge, nt = ge({
        get: Ne,
        set: Ot
      });
      Object.defineProperty(s, Y, {
        enumerable: !0,
        configurable: !0,
        get: () => nt.value,
        set: (Me) => nt.value = Me
      });
    }
  if (l)
    for (const Y in l)
      Lr(l[Y], s, n, Y);
  if (a) {
    const Y = X(a) ? a.call(n) : a;
    Reflect.ownKeys(Y).forEach((Q) => {
      zo(Q, Y[Q]);
    });
  }
  c && ci(c, e, "c");
  function Z(Y, Q) {
    U(Q) ? Q.forEach((Ne) => Y(Ne.bind(n))) : Q && Y(Q.bind(n));
  }
  if (Z(al, h), Z(gt, v), Z(ul, _), Z(cl, C), Z(rl, M), Z(ol, F), Z(pl, H), Z(hl, R), Z(dl, E), Z(xt, L), Z(kr, T), Z(fl, x), U(B))
    if (B.length) {
      const Y = e.exposed || (e.exposed = {});
      B.forEach((Q) => {
        Object.defineProperty(Y, Q, {
          get: () => n[Q],
          set: (Ne) => n[Q] = Ne,
          enumerable: !0
        });
      });
    } else e.exposed || (e.exposed = {});
  K && e.render === Ge && (e.render = K), G != null && (e.inheritAttrs = G), P && (e.components = P), ie && (e.directives = ie), x && Ar(e);
}
function yl(e, t, n = Ge) {
  U(e) && (e = Ss(e));
  for (const s in e) {
    const i = e[s];
    let r;
    le(i) ? "default" in i ? r = Ln(
      i.from || s,
      i.default,
      !0
    ) : r = Ln(i.from || s) : r = Ln(i), /* @__PURE__ */ _e(r) ? Object.defineProperty(t, s, {
      enumerable: !0,
      configurable: !0,
      get: () => r.value,
      set: (o) => r.value = o
    }) : t[s] = r;
  }
}
function ci(e, t, n) {
  Re(
    U(e) ? e.map((s) => s.bind(t.proxy)) : e.bind(t.proxy),
    t,
    n
  );
}
function Lr(e, t, n, s) {
  let i = s.includes(".") ? br(n, s) : () => n[s];
  if (he(e)) {
    const r = t[e];
    X(r) && ln(i, r);
  } else if (X(e))
    ln(i, e.bind(n));
  else if (le(e))
    if (U(e))
      e.forEach((r) => Lr(r, t, n, s));
    else {
      const r = X(e.handler) ? e.handler.bind(n) : t[e.handler];
      X(r) && ln(i, r, e);
    }
}
function Pr(e) {
  const t = e.type, { mixins: n, extends: s } = t, {
    mixins: i,
    optionsCache: r,
    config: { optionMergeStrategies: o }
  } = e.appContext, l = r.get(t);
  let a;
  return l ? a = l : !i.length && !n && !s ? a = t : (a = {}, i.length && i.forEach(
    (d) => Nn(a, d, o, !0)
  ), Nn(a, t, o)), le(t) && r.set(t, a), a;
}
function Nn(e, t, n, s = !1) {
  const { mixins: i, extends: r } = t;
  r && Nn(e, r, n, !0), i && i.forEach(
    (o) => Nn(e, o, n, !0)
  );
  for (const o in t)
    if (!(s && o === "expose")) {
      const l = bl[o] || n && n[o];
      e[o] = l ? l(e[o], t[o]) : t[o];
    }
  return e;
}
const bl = {
  data: fi,
  props: di,
  emits: di,
  // objects
  methods: Qt,
  computed: Qt,
  // lifecycle
  beforeCreate: Te,
  created: Te,
  beforeMount: Te,
  mounted: Te,
  beforeUpdate: Te,
  updated: Te,
  beforeDestroy: Te,
  beforeUnmount: Te,
  destroyed: Te,
  unmounted: Te,
  activated: Te,
  deactivated: Te,
  errorCaptured: Te,
  serverPrefetch: Te,
  // assets
  components: Qt,
  directives: Qt,
  // watch
  watch: _l,
  // provide / inject
  provide: fi,
  inject: wl
};
function fi(e, t) {
  return t ? e ? function() {
    return me(
      X(e) ? e.call(this, this) : e,
      X(t) ? t.call(this, this) : t
    );
  } : t : e;
}
function wl(e, t) {
  return Qt(Ss(e), Ss(t));
}
function Ss(e) {
  if (U(e)) {
    const t = {};
    for (let n = 0; n < e.length; n++)
      t[e[n]] = e[n];
    return t;
  }
  return e;
}
function Te(e, t) {
  return e ? [...new Set([].concat(e, t))] : t;
}
function Qt(e, t) {
  return e ? me(/* @__PURE__ */ Object.create(null), e, t) : t;
}
function di(e, t) {
  return e ? U(e) && U(t) ? [.../* @__PURE__ */ new Set([...e, ...t])] : me(
    /* @__PURE__ */ Object.create(null),
    ui(e),
    ui(t ?? {})
  ) : t;
}
function _l(e, t) {
  if (!e) return t;
  if (!t) return e;
  const n = me(/* @__PURE__ */ Object.create(null), e);
  for (const s in t)
    n[s] = Te(e[s], t[s]);
  return n;
}
function Ir() {
  return {
    app: null,
    config: {
      isNativeTag: Vi,
      performance: !1,
      globalProperties: {},
      optionMergeStrategies: {},
      errorHandler: void 0,
      warnHandler: void 0,
      compilerOptions: {}
    },
    mixins: [],
    components: {},
    directives: {},
    provides: /* @__PURE__ */ Object.create(null),
    optionsCache: /* @__PURE__ */ new WeakMap(),
    propsCache: /* @__PURE__ */ new WeakMap(),
    emitsCache: /* @__PURE__ */ new WeakMap()
  };
}
let xl = 0;
function Tl(e, t) {
  return function(s, i = null) {
    X(s) || (s = me({}, s)), i != null && !le(i) && (i = null);
    const r = Ir(), o = /* @__PURE__ */ new WeakSet(), l = [];
    let a = !1;
    const d = r.app = {
      _uid: xl++,
      _component: s,
      _props: i,
      _container: null,
      _context: r,
      _instance: null,
      version: ta,
      get config() {
        return r.config;
      },
      set config(c) {
      },
      use(c, ...h) {
        return o.has(c) || (c && X(c.install) ? (o.add(c), c.install(d, ...h)) : X(c) && (o.add(c), c(d, ...h))), d;
      },
      mixin(c) {
        return r.mixins.includes(c) || r.mixins.push(c), d;
      },
      component(c, h) {
        return h ? (r.components[c] = h, d) : r.components[c];
      },
      directive(c, h) {
        return h ? (r.directives[c] = h, d) : r.directives[c];
      },
      mount(c, h, v) {
        if (!a) {
          const _ = d._ceVNode || ee(s, i);
          return _.appContext = r, v === !0 ? v = "svg" : v === !1 && (v = void 0), e(_, c, v), a = !0, d._container = c, c.__vue_app__ = d, ts(_.component);
        }
      },
      onUnmount(c) {
        l.push(c);
      },
      unmount() {
        a && (Re(
          l,
          d._instance,
          16
        ), e(null, d._container), delete d._container.__vue_app__);
      },
      provide(c, h) {
        return r.provides[c] = h, d;
      },
      runWithContext(c) {
        const h = Vt;
        Vt = d;
        try {
          return c();
        } finally {
          Vt = h;
        }
      }
    };
    return d;
  };
}
let Vt = null;
const Cl = (e, t) => t === "modelValue" || t === "model-value" ? e.modelModifiers : e[`${t}Modifiers`] || e[`${He(t)}Modifiers`] || e[`${It(t)}Modifiers`];
function El(e, t, ...n) {
  if (e.isUnmounted) return;
  const s = e.vnode.props || ce;
  let i = n;
  const r = t.startsWith("update:"), o = r && Cl(s, t.slice(7));
  o && (o.trim && (i = n.map((c) => he(c) ? c.trim() : c)), o.number && (i = n.map(Is)));
  let l, a = s[l = ss(t)] || // also try camelCase event handler (#2249)
  s[l = ss(He(t))];
  !a && r && (a = s[l = ss(It(t))]), a && Re(
    a,
    e,
    6,
    i
  );
  const d = s[l + "Once"];
  if (d) {
    if (!e.emitted)
      e.emitted = {};
    else if (e.emitted[l])
      return;
    e.emitted[l] = !0, Re(
      d,
      e,
      6,
      i
    );
  }
}
const Sl = /* @__PURE__ */ new WeakMap();
function Or(e, t, n = !1) {
  const s = n ? Sl : t.emitsCache, i = s.get(e);
  if (i !== void 0)
    return i;
  const r = e.emits;
  let o = {}, l = !1;
  if (!X(e)) {
    const a = (d) => {
      const c = Or(d, t, !0);
      c && (l = !0, me(o, c));
    };
    !n && t.mixins.length && t.mixins.forEach(a), e.extends && a(e.extends), e.mixins && e.mixins.forEach(a);
  }
  return !r && !l ? (le(e) && s.set(e, null), null) : (U(r) ? r.forEach((a) => o[a] = null) : me(o, r), le(e) && s.set(e, o), o);
}
function Qn(e, t) {
  return !e || !Wn(t) ? !1 : (t = t.slice(2), t = t === "Once" ? t : t.replace(/Once$/, ""), oe(e, t[0].toLowerCase() + t.slice(1)) || oe(e, It(t)) || oe(e, t));
}
function hi(e) {
  const {
    type: t,
    vnode: n,
    proxy: s,
    withProxy: i,
    propsOptions: [r],
    slots: o,
    attrs: l,
    emit: a,
    render: d,
    renderCache: c,
    props: h,
    data: v,
    setupState: _,
    ctx: C,
    inheritAttrs: M
  } = e, F = Fn(e);
  let D, L;
  try {
    if (n.shapeFlag & 4) {
      const T = i || s, K = T;
      D = Xe(
        d.call(
          K,
          T,
          c,
          h,
          _,
          v,
          C
        )
      ), L = l;
    } else {
      const T = t;
      D = Xe(
        T.length > 1 ? T(
          h,
          { attrs: l, slots: o, emit: a }
        ) : T(
          h,
          null
        )
      ), L = t.props ? l : $l(l);
    }
  } catch (T) {
    fn.length = 0, Gn(T, e, 1), D = ee(Se);
  }
  let N = D;
  if (L && M !== !1) {
    const T = Object.keys(L), { shapeFlag: K } = N;
    T.length && K & 7 && (r && T.some(qn) && (L = Al(
      L,
      r
    )), N = _t(N, L, !1, !0));
  }
  return n.dirs && (N = _t(N, null, !1, !0), N.dirs = N.dirs ? N.dirs.concat(n.dirs) : n.dirs), n.transition && gn(N, n.transition), D = N, Fn(F), D;
}
const $l = (e) => {
  let t;
  for (const n in e)
    (n === "class" || n === "style" || Wn(n)) && ((t || (t = {}))[n] = e[n]);
  return t;
}, Al = (e, t) => {
  const n = {};
  for (const s in e)
    (!qn(s) || !(s.slice(9) in t)) && (n[s] = e[s]);
  return n;
};
function Ml(e, t, n) {
  const { props: s, children: i, component: r } = e, { props: o, children: l, patchFlag: a } = t, d = r.emitsOptions;
  if (t.dirs || t.transition)
    return !0;
  if (n && a >= 0) {
    if (a & 1024)
      return !0;
    if (a & 16)
      return s ? pi(s, o, d) : !!o;
    if (a & 8) {
      const c = t.dynamicProps;
      for (let h = 0; h < c.length; h++) {
        const v = c[h];
        if (Dr(o, s, v) && !Qn(d, v))
          return !0;
      }
    }
  } else
    return (i || l) && (!l || !l.$stable) ? !0 : s === o ? !1 : s ? o ? pi(s, o, d) : !0 : !!o;
  return !1;
}
function pi(e, t, n) {
  const s = Object.keys(t);
  if (s.length !== Object.keys(e).length)
    return !0;
  for (let i = 0; i < s.length; i++) {
    const r = s[i];
    if (Dr(t, e, r) && !Qn(n, r))
      return !0;
  }
  return !1;
}
function Dr(e, t, n) {
  const s = e[n], i = t[n];
  return n === "style" && le(s) && le(i) ? !Os(s, i) : s !== i;
}
function kl({ vnode: e, parent: t, suspense: n }, s) {
  for (; t; ) {
    const i = t.subTree;
    if (i.suspense && i.suspense.activeBranch === e && (i.suspense.vnode.el = i.el = s, e = i), i === e)
      (e = t.vnode).el = s, t = t.parent;
    else
      break;
  }
  n && n.activeBranch === e && (n.vnode.el = s);
}
const Fr = {}, Rr = () => Object.create(Fr), Nr = (e) => Object.getPrototypeOf(e) === Fr;
function Ll(e, t, n, s = !1) {
  const i = {}, r = Rr();
  e.propsDefaults = /* @__PURE__ */ Object.create(null), Hr(e, t, i, r);
  for (const o in e.propsOptions[0])
    o in i || (i[o] = void 0);
  n ? e.props = s ? i : /* @__PURE__ */ Ro(i) : e.type.props ? e.props = i : e.props = r, e.attrs = r;
}
function Pl(e, t, n, s) {
  const {
    props: i,
    attrs: r,
    vnode: { patchFlag: o }
  } = e, l = /* @__PURE__ */ re(i), [a] = e.propsOptions;
  let d = !1;
  if (
    // always force full diff in dev
    // - #1942 if hmr is enabled with sfc component
    // - vite#872 non-sfc component used by sfc component
    (s || o > 0) && !(o & 16)
  ) {
    if (o & 8) {
      const c = e.vnode.dynamicProps;
      for (let h = 0; h < c.length; h++) {
        let v = c[h];
        if (Qn(e.emitsOptions, v))
          continue;
        const _ = t[v];
        if (a)
          if (oe(r, v))
            _ !== r[v] && (r[v] = _, d = !0);
          else {
            const C = He(v);
            i[C] = $s(
              a,
              l,
              C,
              _,
              e,
              !1
            );
          }
        else
          _ !== r[v] && (r[v] = _, d = !0);
      }
    }
  } else {
    Hr(e, t, i, r) && (d = !0);
    let c;
    for (const h in l)
      (!t || // for camelCase
      !oe(t, h) && // it's possible the original props was passed in as kebab-case
      // and converted to camelCase (#955)
      ((c = It(h)) === h || !oe(t, c))) && (a ? n && // for camelCase
      (n[h] !== void 0 || // for kebab-case
      n[c] !== void 0) && (i[h] = $s(
        a,
        l,
        h,
        void 0,
        e,
        !0
      )) : delete i[h]);
    if (r !== l)
      for (const h in r)
        (!t || !oe(t, h)) && (delete r[h], d = !0);
  }
  d && ut(e.attrs, "set", "");
}
function Hr(e, t, n, s) {
  const [i, r] = e.propsOptions;
  let o = !1, l;
  if (t)
    for (let a in t) {
      if (sn(a))
        continue;
      const d = t[a];
      let c;
      i && oe(i, c = He(a)) ? !r || !r.includes(c) ? n[c] = d : (l || (l = {}))[c] = d : Qn(e.emitsOptions, a) || (!(a in s) || d !== s[a]) && (s[a] = d, o = !0);
    }
  if (r) {
    const a = /* @__PURE__ */ re(n), d = l || ce;
    for (let c = 0; c < r.length; c++) {
      const h = r[c];
      n[h] = $s(
        i,
        a,
        h,
        d[h],
        e,
        !oe(d, h)
      );
    }
  }
  return o;
}
function $s(e, t, n, s, i, r) {
  const o = e[n];
  if (o != null) {
    const l = oe(o, "default");
    if (l && s === void 0) {
      const a = o.default;
      if (o.type !== Function && !o.skipFactory && X(a)) {
        const { propsDefaults: d } = i;
        if (n in d)
          s = d[n];
        else {
          const c = _n(i);
          s = d[n] = a.call(
            null,
            t
          ), c();
        }
      } else
        s = a;
      i.ce && i.ce._setProp(n, s);
    }
    o[
      0
      /* shouldCast */
    ] && (r && !l ? s = !1 : o[
      1
      /* shouldCastTrue */
    ] && (s === "" || s === It(n)) && (s = !0));
  }
  return s;
}
const Il = /* @__PURE__ */ new WeakMap();
function Br(e, t, n = !1) {
  const s = n ? Il : t.propsCache, i = s.get(e);
  if (i)
    return i;
  const r = e.props, o = {}, l = [];
  let a = !1;
  if (!X(e)) {
    const c = (h) => {
      a = !0;
      const [v, _] = Br(h, t, !0);
      me(o, v), _ && l.push(..._);
    };
    !n && t.mixins.length && t.mixins.forEach(c), e.extends && c(e.extends), e.mixins && e.mixins.forEach(c);
  }
  if (!r && !a)
    return le(e) && s.set(e, Nt), Nt;
  if (U(r))
    for (let c = 0; c < r.length; c++) {
      const h = He(r[c]);
      gi(h) && (o[h] = ce);
    }
  else if (r)
    for (const c in r) {
      const h = He(c);
      if (gi(h)) {
        const v = r[c], _ = o[h] = U(v) || X(v) ? { type: v } : me({}, v), C = _.type;
        let M = !1, F = !0;
        if (U(C))
          for (let D = 0; D < C.length; ++D) {
            const L = C[D], N = X(L) && L.name;
            if (N === "Boolean") {
              M = !0;
              break;
            } else N === "String" && (F = !1);
          }
        else
          M = X(C) && C.name === "Boolean";
        _[
          0
          /* shouldCast */
        ] = M, _[
          1
          /* shouldCastTrue */
        ] = F, (M || oe(_, "default")) && l.push(h);
      }
    }
  const d = [o, l];
  return le(e) && s.set(e, d), d;
}
function gi(e) {
  return e[0] !== "$" && !sn(e);
}
const Us = (e) => e === "_" || e === "_ctx" || e === "$stable", Ws = (e) => U(e) ? e.map(Xe) : [Xe(e)], Ol = (e, t, n) => {
  if (t._n)
    return t;
  const s = Ut((...i) => Ws(t(...i)), n);
  return s._c = !1, s;
}, Vr = (e, t, n) => {
  const s = e._ctx;
  for (const i in e) {
    if (Us(i)) continue;
    const r = e[i];
    if (X(r))
      t[i] = Ol(i, r, s);
    else if (r != null) {
      const o = Ws(r);
      t[i] = () => o;
    }
  }
}, jr = (e, t) => {
  const n = Ws(t);
  e.slots.default = () => n;
}, Kr = (e, t, n) => {
  for (const s in t)
    (n || !Us(s)) && (e[s] = t[s]);
}, Dl = (e, t, n) => {
  const s = e.slots = Rr();
  if (e.vnode.shapeFlag & 32) {
    const i = t._;
    i ? (Kr(s, t, n), n && Yi(s, "_", i, !0)) : Vr(t, s);
  } else t && jr(e, t);
}, Fl = (e, t, n) => {
  const { vnode: s, slots: i } = e;
  let r = !0, o = ce;
  if (s.shapeFlag & 32) {
    const l = t._;
    l ? n && l === 1 ? r = !1 : Kr(i, t, n) : (r = !t.$stable, Vr(t, i)), o = t;
  } else t && (jr(e, t), o = { default: 1 });
  if (r)
    for (const l in i)
      !Us(l) && o[l] == null && delete i[l];
}, Ce = Vl;
function Rl(e) {
  return Nl(e);
}
function Nl(e, t) {
  const n = Xn();
  n.__VUE__ = !0;
  const {
    insert: s,
    remove: i,
    patchProp: r,
    createElement: o,
    createText: l,
    createComment: a,
    setText: d,
    setElementText: c,
    parentNode: h,
    nextSibling: v,
    setScopeId: _ = Ge,
    insertStaticContent: C
  } = e, M = (u, f, p, m = null, b = null, y = null, k = void 0, A = null, $ = !!f.dynamicChildren) => {
    if (u === f)
      return;
    u && !kt(u, f) && (m = mt(u), Me(u, b, y, !0), u = null), f.patchFlag === -2 && ($ = !1, f.dynamicChildren = null);
    const { type: w, ref: V, shapeFlag: O } = f;
    switch (w) {
      case es:
        F(u, f, p, m);
        break;
      case Se:
        D(u, f, p, m);
        break;
      case fs:
        u == null && L(f, p, m, k);
        break;
      case te:
        P(
          u,
          f,
          p,
          m,
          b,
          y,
          k,
          A,
          $
        );
        break;
      default:
        O & 1 ? K(
          u,
          f,
          p,
          m,
          b,
          y,
          k,
          A,
          $
        ) : O & 6 ? ie(
          u,
          f,
          p,
          m,
          b,
          y,
          k,
          A,
          $
        ) : (O & 64 || O & 128) && w.process(
          u,
          f,
          p,
          m,
          b,
          y,
          k,
          A,
          $,
          it
        );
    }
    V != null && b ? an(V, u && u.ref, y, f || u, !f) : V == null && u && u.ref != null && an(u.ref, null, y, u, !0);
  }, F = (u, f, p, m) => {
    if (u == null)
      s(
        f.el = l(f.children),
        p,
        m
      );
    else {
      const b = f.el = u.el;
      f.children !== u.children && d(b, f.children);
    }
  }, D = (u, f, p, m) => {
    u == null ? s(
      f.el = a(f.children || ""),
      p,
      m
    ) : f.el = u.el;
  }, L = (u, f, p, m) => {
    [u.el, u.anchor] = C(
      u.children,
      f,
      p,
      m,
      u.el,
      u.anchor
    );
  }, N = ({ el: u, anchor: f }, p, m) => {
    let b;
    for (; u && u !== f; )
      b = v(u), s(u, p, m), u = b;
    s(f, p, m);
  }, T = ({ el: u, anchor: f }) => {
    let p;
    for (; u && u !== f; )
      p = v(u), i(u), u = p;
    i(f);
  }, K = (u, f, p, m, b, y, k, A, $) => {
    if (f.type === "svg" ? k = "svg" : f.type === "math" && (k = "mathml"), u == null)
      R(
        f,
        p,
        m,
        b,
        y,
        k,
        A,
        $
      );
    else {
      const w = u.el && u.el._isVueCE ? u.el : null;
      try {
        w && w._beginPatch(), x(
          u,
          f,
          b,
          y,
          k,
          A,
          $
        );
      } finally {
        w && w._endPatch();
      }
    }
  }, R = (u, f, p, m, b, y, k, A) => {
    let $, w;
    const { props: V, shapeFlag: O, transition: j, dirs: q } = u;
    if ($ = u.el = o(
      u.type,
      y,
      V && V.is,
      V
    ), O & 8 ? c($, u.children) : O & 16 && H(
      u.children,
      $,
      null,
      m,
      b,
      cs(u, y),
      k,
      A
    ), q && Ct(u, null, m, "created"), E($, u, u.scopeId, k, m), V) {
      for (const ue in V)
        ue !== "value" && !sn(ue) && r($, ue, null, V[ue], y, m);
      "value" in V && r($, "value", null, V.value, y), (w = V.onVnodeBeforeMount) && We(w, m, u);
    }
    q && Ct(u, null, m, "beforeMount");
    const se = Hl(b, j);
    se && j.beforeEnter($), s($, f, p), ((w = V && V.onVnodeMounted) || se || q) && Ce(() => {
      w && We(w, m, u), se && j.enter($), q && Ct(u, null, m, "mounted");
    }, b);
  }, E = (u, f, p, m, b) => {
    if (p && _(u, p), m)
      for (let y = 0; y < m.length; y++)
        _(u, m[y]);
    if (b) {
      let y = b.subTree;
      if (f === y || qr(y.type) && (y.ssContent === f || y.ssFallback === f)) {
        const k = b.vnode;
        E(
          u,
          k,
          k.scopeId,
          k.slotScopeIds,
          b.parent
        );
      }
    }
  }, H = (u, f, p, m, b, y, k, A, $ = 0) => {
    for (let w = $; w < u.length; w++) {
      const V = u[w] = A ? at(u[w]) : Xe(u[w]);
      M(
        null,
        V,
        f,
        p,
        m,
        b,
        y,
        k,
        A
      );
    }
  }, x = (u, f, p, m, b, y, k) => {
    const A = f.el = u.el;
    let { patchFlag: $, dynamicChildren: w, dirs: V } = f;
    $ |= u.patchFlag & 16;
    const O = u.props || ce, j = f.props || ce;
    let q;
    if (p && Et(p, !1), (q = j.onVnodeBeforeUpdate) && We(q, p, f, u), V && Ct(f, u, p, "beforeUpdate"), p && Et(p, !0), // #6385 the old vnode may be a user-wrapped non-isomorphic block
    // Force full diff when block metadata is unstable.
    w && (!u.dynamicChildren || u.dynamicChildren.length !== w.length) && ($ = 0, k = !1, w = null), (O.innerHTML && j.innerHTML == null || O.textContent && j.textContent == null) && c(A, ""), w ? B(
      u.dynamicChildren,
      w,
      A,
      p,
      m,
      cs(f, b),
      y
    ) : k || Q(
      u,
      f,
      A,
      null,
      p,
      m,
      cs(f, b),
      y,
      !1
    ), $ > 0) {
      if ($ & 16)
        G(A, O, j, p, b);
      else if ($ & 2 && O.class !== j.class && r(A, "class", null, j.class, b), $ & 4 && r(A, "style", O.style, j.style, b), $ & 8) {
        const se = f.dynamicProps;
        for (let ue = 0; ue < se.length; ue++) {
          const ae = se[ue], pe = O[ae], ye = j[ae];
          (ye !== pe || ae === "value") && r(A, ae, pe, ye, b, p);
        }
      }
      $ & 1 && u.children !== f.children && c(A, f.children);
    } else !k && w == null && G(A, O, j, p, b);
    ((q = j.onVnodeUpdated) || V) && Ce(() => {
      q && We(q, p, f, u), V && Ct(f, u, p, "updated");
    }, m);
  }, B = (u, f, p, m, b, y, k) => {
    for (let A = 0; A < f.length; A++) {
      const $ = u[A], w = f[A], V = (
        // oldVNode may be an errored async setup() component inside Suspense
        // which will not have a mounted element
        $.el && // - In the case of a Fragment, we need to provide the actual parent
        // of the Fragment itself so it can move its children.
        ($.type === te || // - In the case of different nodes, there is going to be a replacement
        // which also requires the correct parent container
        !kt($, w) || // - In the case of a component, it could contain anything.
        $.shapeFlag & 198) ? h($.el) : (
          // In other cases, the parent container is not actually used so we
          // just pass the block element here to avoid a DOM parentNode call.
          p
        )
      );
      M(
        $,
        w,
        V,
        null,
        m,
        b,
        y,
        k,
        !0
      );
    }
  }, G = (u, f, p, m, b) => {
    if (f !== p) {
      if (f !== ce)
        for (const y in f)
          !sn(y) && !(y in p) && r(
            u,
            y,
            f[y],
            null,
            b,
            m
          );
      for (const y in p) {
        if (sn(y)) continue;
        const k = p[y], A = f[y];
        k !== A && y !== "value" && r(u, y, A, k, b, m);
      }
      "value" in p && r(u, "value", f.value, p.value, b);
    }
  }, P = (u, f, p, m, b, y, k, A, $) => {
    const w = f.el = u ? u.el : l(""), V = f.anchor = u ? u.anchor : l("");
    let { patchFlag: O, dynamicChildren: j, slotScopeIds: q } = f;
    q && (A = A ? A.concat(q) : q), u == null ? (s(w, p, m), s(V, p, m), H(
      // #10007
      // such fragment like `<></>` will be compiled into
      // a fragment which doesn't have a children.
      // In this case fallback to an empty array
      f.children || [],
      p,
      V,
      b,
      y,
      k,
      A,
      $
    )) : O > 0 && O & 64 && j && // #2715 the previous fragment could've been a BAILed one as a result
    // of renderSlot() with no valid children
    u.dynamicChildren && u.dynamicChildren.length === j.length ? (B(
      u.dynamicChildren,
      j,
      p,
      b,
      y,
      k,
      A
    ), // #2080 if the stable fragment has a key, it's a <template v-for> that may
    //  get moved around. Make sure all root level vnodes inherit el.
    // #2134 or if it's a component root, it may also get moved around
    // as the component is being moved.
    (f.key != null || b && f === b.subTree) && qs(
      u,
      f,
      !0
      /* shallow */
    )) : Q(
      u,
      f,
      p,
      V,
      b,
      y,
      k,
      A,
      $
    );
  }, ie = (u, f, p, m, b, y, k, A, $) => {
    f.slotScopeIds = A, u == null ? f.shapeFlag & 512 ? b.ctx.activate(
      f,
      p,
      m,
      k,
      $
    ) : W(
      f,
      p,
      m,
      b,
      y,
      k,
      $
    ) : J(u, f, $);
  }, W = (u, f, p, m, b, y, k) => {
    const A = u.component = Xl(
      u,
      m,
      b
    );
    if (Jn(u) && (A.ctx.renderer = it), zl(A, !1, k), A.asyncDep) {
      if (b && b.registerDep(A, Z, k), !u.el) {
        const $ = A.subTree = ee(Se);
        D(null, $, f, p), u.placeholder = $.el;
      }
    } else
      Z(
        A,
        u,
        f,
        p,
        b,
        y,
        k
      );
  }, J = (u, f, p) => {
    const m = f.component = u.component;
    if (Ml(u, f, p))
      if (m.asyncDep && !m.asyncResolved) {
        Y(m, f, p);
        return;
      } else
        m.next = f, m.update();
    else
      f.el = u.el, m.vnode = f;
  }, Z = (u, f, p, m, b, y, k) => {
    const A = () => {
      if (u.isMounted) {
        let { next: O, bu: j, u: q, parent: se, vnode: ue } = u;
        {
          const Ke = Ur(u);
          if (Ke) {
            O && (O.el = ue.el, Y(u, O, k)), Ke.asyncDep.then(() => {
              Ce(() => {
                u.isUnmounted || w();
              }, b);
            });
            return;
          }
        }
        let ae = O, pe;
        Et(u, !1), O ? (O.el = ue.el, Y(u, O, k)) : O = ue, j && kn(j), (pe = O.props && O.props.onVnodeBeforeUpdate) && We(pe, se, O, ue), Et(u, !0);
        const ye = hi(u), je = u.subTree;
        u.subTree = ye, M(
          je,
          ye,
          // parent may have changed if it's in a teleport
          h(je.el),
          // anchor may have changed if it's in a fragment
          mt(je),
          u,
          b,
          y
        ), O.el = ye.el, ae === null && kl(u, ye.el), q && Ce(q, b), (pe = O.props && O.props.onVnodeUpdated) && Ce(
          () => We(pe, se, O, ue),
          b
        );
      } else {
        let O;
        const { el: j, props: q } = f, { bm: se, m: ue, parent: ae, root: pe, type: ye } = u, je = un(f);
        Et(u, !1), se && kn(se), !je && (O = q && q.onVnodeBeforeMount) && We(O, ae, f), Et(u, !0);
        {
          pe.ce && pe.ce._hasShadowRoot() && pe.ce._injectChildStyle(
            ye,
            u.parent ? u.parent.type : void 0
          );
          const Ke = u.subTree = hi(u);
          M(
            null,
            Ke,
            p,
            m,
            u,
            b,
            y
          ), f.el = Ke.el;
        }
        if (ue && Ce(ue, b), !je && (O = q && q.onVnodeMounted)) {
          const Ke = f;
          Ce(
            () => We(O, ae, Ke),
            b
          );
        }
        (f.shapeFlag & 256 || ae && un(ae.vnode) && ae.vnode.shapeFlag & 256) && u.a && Ce(u.a, b), u.isMounted = !0, f = p = m = null;
      }
    };
    u.scope.on();
    const $ = u.effect = new Ji(A);
    u.scope.off();
    const w = u.update = $.run.bind($), V = u.job = $.runIfDirty.bind($);
    V.i = u, V.id = u.uid, $.scheduler = () => Ks(V), Et(u, !0), w();
  }, Y = (u, f, p) => {
    f.component = u;
    const m = u.vnode.props;
    u.vnode = f, u.next = null, Pl(u, f.props, m, p), Fl(u, f.children, p), Ze(), si(u), Qe();
  }, Q = (u, f, p, m, b, y, k, A, $ = !1) => {
    const w = u && u.children, V = u ? u.shapeFlag : 0, O = f.children, { patchFlag: j, shapeFlag: q } = f;
    if (j > 0) {
      if (j & 128) {
        Ot(
          w,
          O,
          p,
          m,
          b,
          y,
          k,
          A,
          $
        );
        return;
      } else if (j & 256) {
        Ne(
          w,
          O,
          p,
          m,
          b,
          y,
          k,
          A,
          $
        );
        return;
      }
    }
    q & 8 ? (V & 16 && Tt(w, b, y), O !== w && c(p, O)) : V & 16 ? q & 16 ? Ot(
      w,
      O,
      p,
      m,
      b,
      y,
      k,
      A,
      $
    ) : Tt(w, b, y, !0) : (V & 8 && c(p, ""), q & 16 && H(
      O,
      p,
      m,
      b,
      y,
      k,
      A,
      $
    ));
  }, Ne = (u, f, p, m, b, y, k, A, $) => {
    u = u || Nt, f = f || Nt;
    const w = u.length, V = f.length, O = Math.min(w, V);
    let j;
    for (j = 0; j < O; j++) {
      const q = f[j] = $ ? at(f[j]) : Xe(f[j]);
      M(
        u[j],
        q,
        p,
        null,
        b,
        y,
        k,
        A,
        $
      );
    }
    w > V ? Tt(
      u,
      b,
      y,
      !0,
      !1,
      O
    ) : H(
      f,
      p,
      m,
      b,
      y,
      k,
      A,
      $,
      O
    );
  }, Ot = (u, f, p, m, b, y, k, A, $) => {
    let w = 0;
    const V = f.length;
    let O = u.length - 1, j = V - 1;
    for (; w <= O && w <= j; ) {
      const q = u[w], se = f[w] = $ ? at(f[w]) : Xe(f[w]);
      if (kt(q, se))
        M(
          q,
          se,
          p,
          null,
          b,
          y,
          k,
          A,
          $
        );
      else
        break;
      w++;
    }
    for (; w <= O && w <= j; ) {
      const q = u[O], se = f[j] = $ ? at(f[j]) : Xe(f[j]);
      if (kt(q, se))
        M(
          q,
          se,
          p,
          null,
          b,
          y,
          k,
          A,
          $
        );
      else
        break;
      O--, j--;
    }
    if (w > O) {
      if (w <= j) {
        const q = j + 1, se = q < V ? f[q].el : m;
        for (; w <= j; )
          M(
            null,
            f[w] = $ ? at(f[w]) : Xe(f[w]),
            p,
            se,
            b,
            y,
            k,
            A,
            $
          ), w++;
      }
    } else if (w > j)
      for (; w <= O; )
        Me(u[w], b, y, !0), w++;
    else {
      const q = w, se = w, ue = /* @__PURE__ */ new Map();
      for (w = se; w <= j; w++) {
        const ke = f[w] = $ ? at(f[w]) : Xe(f[w]);
        ke.key != null && ue.set(ke.key, w);
      }
      let ae, pe = 0;
      const ye = j - se + 1;
      let je = !1, Ke = 0;
      const Yt = new Array(ye);
      for (w = 0; w < ye; w++) Yt[w] = 0;
      for (w = q; w <= O; w++) {
        const ke = u[w];
        if (pe >= ye) {
          Me(ke, b, y, !0);
          continue;
        }
        let Ue;
        if (ke.key != null)
          Ue = ue.get(ke.key);
        else
          for (ae = se; ae <= j; ae++)
            if (Yt[ae - se] === 0 && kt(ke, f[ae])) {
              Ue = ae;
              break;
            }
        Ue === void 0 ? Me(ke, b, y, !0) : (Yt[Ue - se] = w + 1, Ue >= Ke ? Ke = Ue : je = !0, M(
          ke,
          f[Ue],
          p,
          null,
          b,
          y,
          k,
          A,
          $
        ), pe++);
      }
      const Gs = je ? Bl(Yt) : Nt;
      for (ae = Gs.length - 1, w = ye - 1; w >= 0; w--) {
        const ke = se + w, Ue = f[ke], Js = f[ke + 1], Zs = ke + 1 < V ? (
          // #13559, #14173 fallback to el placeholder for unresolved async component
          Js.el || Wr(Js)
        ) : m;
        Yt[w] === 0 ? M(
          null,
          Ue,
          p,
          Zs,
          b,
          y,
          k,
          A,
          $
        ) : je && (ae < 0 || w !== Gs[ae] ? nt(Ue, p, Zs, 2) : ae--);
      }
    }
  }, nt = (u, f, p, m, b = null) => {
    const { el: y, type: k, transition: A, children: $, shapeFlag: w } = u;
    if (w & 6) {
      nt(u.component.subTree, f, p, m);
      return;
    }
    if (w & 128) {
      u.suspense.move(f, p, m);
      return;
    }
    if (w & 64) {
      k.move(u, f, p, it);
      return;
    }
    if (k === te) {
      s(y, f, p);
      for (let O = 0; O < $.length; O++)
        nt($[O], f, p, m);
      s(u.anchor, f, p);
      return;
    }
    if (k === fs) {
      N(u, f, p);
      return;
    }
    if (m !== 2 && w & 1 && A)
      if (m === 0)
        A.persisted && !y[Oe] ? s(y, f, p) : (A.beforeEnter(y), s(y, f, p), Ce(() => A.enter(y), b));
      else {
        const { leave: O, delayLeave: j, afterLeave: q } = A, se = () => {
          u.ctx.isUnmounted ? i(y) : s(y, f, p);
        }, ue = () => {
          const ae = y._isLeaving || !!y[Oe];
          y._isLeaving && y[Oe](
            !0
            /* cancelled */
          ), A.persisted && !ae ? se() : O(y, () => {
            se(), q && q();
          });
        };
        j ? j(y, se, ue) : ue();
      }
    else
      s(y, f, p);
  }, Me = (u, f, p, m = !1, b = !1) => {
    const {
      type: y,
      props: k,
      ref: A,
      children: $,
      dynamicChildren: w,
      shapeFlag: V,
      patchFlag: O,
      dirs: j,
      cacheIndex: q,
      memo: se
    } = u;
    if (O === -2 && (b = !1), A != null && (Ze(), an(A, null, p, u, !0), Qe()), q != null && (f.renderCache[q] = void 0), V & 256) {
      f.ctx.deactivate(u);
      return;
    }
    const ue = V & 1 && j, ae = !un(u);
    let pe;
    if (ae && (pe = k && k.onVnodeBeforeUnmount) && We(pe, f, u), V & 6)
      ns(u.component, p, m);
    else {
      if (V & 128) {
        u.suspense.unmount(p, m);
        return;
      }
      ue && Ct(u, null, f, "beforeUnmount"), V & 64 ? u.type.remove(
        u,
        f,
        p,
        it,
        m
      ) : w && // #5154
      // when v-once is used inside a block, setBlockTracking(-1) marks the
      // parent block with hasOnce: true
      // so that it doesn't take the fast path during unmount - otherwise
      // components nested in v-once are never unmounted.
      !w.hasOnce && // #1153: fast path should not be taken for non-stable (v-for) fragments
      (y !== te || O > 0 && O & 64) ? Tt(
        w,
        f,
        p,
        !1,
        !0
      ) : (y === te && O & 384 || !b && V & 16) && Tt($, f, p), m && Tn(u);
    }
    const ye = se != null && q == null;
    (ae && (pe = k && k.onVnodeUnmounted) || ue || ye) && Ce(() => {
      pe && We(pe, f, u), ue && Ct(u, null, f, "unmounted"), ye && (u.el = null);
    }, p);
  }, Tn = (u) => {
    const { type: f, el: p, anchor: m, transition: b } = u;
    if (f === te) {
      Cn(p, m);
      return;
    }
    if (f === fs) {
      T(u);
      return;
    }
    const y = () => {
      i(p), b && !b.persisted && b.afterLeave && b.afterLeave();
    };
    if (u.shapeFlag & 1 && b && !b.persisted) {
      const { leave: k, delayLeave: A } = b, $ = () => k(p, y);
      A ? A(u.el, y, $) : $();
    } else
      y();
  }, Cn = (u, f) => {
    let p;
    for (; u !== f; )
      p = v(u), i(u), u = p;
    i(f);
  }, ns = (u, f, p) => {
    const { bum: m, scope: b, job: y, subTree: k, um: A, m: $, a: w } = u;
    mi($), mi(w), m && kn(m), b.stop(), y && (y.flags |= 8, Me(k, u, f, p)), A && Ce(A, f), Ce(() => {
      u.isUnmounted = !0;
    }, f);
  }, Tt = (u, f, p, m = !1, b = !1, y = 0) => {
    for (let k = y; k < u.length; k++)
      Me(u[k], f, p, m, b);
  }, mt = (u) => {
    if (u.shapeFlag & 6)
      return mt(u.component.subTree);
    if (u.shapeFlag & 128)
      return u.suspense.next();
    const f = v(u.anchor || u.el), p = f && f[wr];
    return p ? v(p) : f;
  };
  let st = !1;
  const qt = (u, f, p) => {
    let m;
    u == null ? f._vnode && (Me(f._vnode, null, null, !0), m = f._vnode.component) : M(
      f._vnode || null,
      u,
      f,
      null,
      null,
      null,
      p
    ), f._vnode = u, st || (st = !0, si(m), pr(), st = !1);
  }, it = {
    p: M,
    um: Me,
    m: nt,
    r: Tn,
    mt: W,
    mc: H,
    pc: Q,
    pbc: B,
    n: mt,
    o: e
  };
  return {
    render: qt,
    hydrate: void 0,
    createApp: Tl(qt)
  };
}
function cs({ type: e, props: t }, n) {
  return n === "svg" && e === "foreignObject" || n === "mathml" && e === "annotation-xml" && t && t.encoding && t.encoding.includes("html") ? void 0 : n;
}
function Et({ effect: e, job: t }, n) {
  n ? (e.flags |= 32, t.flags |= 4) : (e.flags &= -33, t.flags &= -5);
}
function Hl(e, t) {
  return (!e || e && !e.pendingBranch) && t && !t.persisted;
}
function qs(e, t, n = !1) {
  const s = e.children, i = t.children;
  if (U(s) && U(i))
    for (let r = 0; r < s.length; r++) {
      const o = s[r];
      let l = i[r];
      l.shapeFlag & 1 && !l.dynamicChildren && ((l.patchFlag <= 0 || l.patchFlag === 32) && (l = i[r] = at(i[r]), l.el = o.el), !n && l.patchFlag !== -2 && qs(o, l)), l.type === es && (l.patchFlag === -1 && (l = i[r] = at(l)), l.el = o.el), l.type === Se && !l.el && (l.el = o.el);
    }
}
function Bl(e) {
  const t = e.slice(), n = [0];
  let s, i, r, o, l;
  const a = e.length;
  for (s = 0; s < a; s++) {
    const d = e[s];
    if (d !== 0) {
      if (i = n[n.length - 1], e[i] < d) {
        t[s] = i, n.push(s);
        continue;
      }
      for (r = 0, o = n.length - 1; r < o; )
        l = r + o >> 1, e[n[l]] < d ? r = l + 1 : o = l;
      d < e[n[r]] && (r > 0 && (t[s] = n[r - 1]), n[r] = s);
    }
  }
  for (r = n.length, o = n[r - 1]; r-- > 0; )
    n[r] = o, o = t[o];
  return n;
}
function Ur(e) {
  const t = e.subTree.component;
  if (t)
    return t.asyncDep && !t.asyncResolved ? t : Ur(t);
}
function mi(e) {
  if (e)
    for (let t = 0; t < e.length; t++)
      e[t].flags |= 8;
}
function Wr(e) {
  if (e.placeholder)
    return e.placeholder;
  const t = e.component;
  return t ? Wr(t.subTree) : null;
}
const qr = (e) => e.__isSuspense;
function Vl(e, t) {
  t && t.pendingBranch ? U(e) ? t.effects.push(...e) : t.effects.push(e) : Xo(e);
}
const te = /* @__PURE__ */ Symbol.for("v-fgt"), es = /* @__PURE__ */ Symbol.for("v-txt"), Se = /* @__PURE__ */ Symbol.for("v-cmt"), fs = /* @__PURE__ */ Symbol.for("v-stc"), fn = [];
let Pe = null;
function S(e = !1) {
  fn.push(Pe = e ? null : []);
}
function jl() {
  fn.pop(), Pe = fn[fn.length - 1] || null;
}
let mn = 1;
function Hn(e, t = !1) {
  mn += e, e < 0 && Pe && t && (Pe.hasOnce = !0);
}
function Yr(e) {
  return e.dynamicChildren = mn > 0 ? Pe || Nt : null, jl(), mn > 0 && Pe && Pe.push(e), e;
}
function I(e, t, n, s, i, r) {
  return Yr(
    g(
      e,
      t,
      n,
      s,
      i,
      r,
      !0
    )
  );
}
function et(e, t, n, s, i) {
  return Yr(
    ee(
      e,
      t,
      n,
      s,
      i,
      !0
    )
  );
}
function Bn(e) {
  return e ? e.__v_isVNode === !0 : !1;
}
function kt(e, t) {
  return e.type === t.type && e.key === t.key;
}
const Xr = ({ key: e }) => e ?? null, Pn = ({
  ref: e,
  ref_key: t,
  ref_for: n
}) => (typeof e == "number" && (e = "" + e), e != null ? he(e) || /* @__PURE__ */ _e(e) || X(e) ? { i: De, r: e, k: t, f: !!n } : e : null);
function g(e, t = null, n = null, s = 0, i = null, r = e === te ? 0 : 1, o = !1, l = !1) {
  const a = {
    __v_isVNode: !0,
    __v_skip: !0,
    type: e,
    props: t,
    key: t && Xr(t),
    ref: t && Pn(t),
    scopeId: mr,
    slotScopeIds: null,
    children: n,
    component: null,
    suspense: null,
    ssContent: null,
    ssFallback: null,
    dirs: null,
    transition: null,
    el: null,
    anchor: null,
    target: null,
    targetStart: null,
    targetAnchor: null,
    staticCount: 0,
    shapeFlag: r,
    patchFlag: s,
    dynamicProps: i,
    dynamicChildren: null,
    appContext: null,
    ctx: De
  };
  return l ? (Vn(a, n), r & 128 && e.normalize(a)) : n && (a.shapeFlag |= he(n) ? 8 : 16), mn > 0 && // avoid a block node from tracking itself
  !o && // has current parent block
  Pe && // presence of a patch flag indicates this node needs patching on updates.
  // component nodes also should always be patched, because even if the
  // component doesn't need to update, it needs to persist the instance on to
  // the next vnode so that it can be properly unmounted later.
  (a.patchFlag > 0 || r & 6) && // the EVENTS flag is only for hydration and if it is the only flag, the
  // vnode should not be considered dynamic due to handler caching.
  a.patchFlag !== 32 && Pe.push(a), a;
}
const ee = Kl;
function Kl(e, t = null, n = null, s = 0, i = null, r = !1) {
  if ((!e || e === gl) && (e = Se), Bn(e)) {
    const l = _t(
      e,
      t,
      !0
      /* mergeRef: true */
    );
    return n && Vn(l, n), mn > 0 && !r && Pe && (l.shapeFlag & 6 ? Pe[Pe.indexOf(e)] = l : Pe.push(l)), l.patchFlag = -2, l;
  }
  if (Ql(e) && (e = e.__vccOpts), t) {
    t = Ul(t);
    let { class: l, style: a } = t;
    l && !he(l) && (t.class = Ae(l)), le(a) && (/* @__PURE__ */ js(a) && !U(a) && (a = me({}, a)), t.style = Kt(a));
  }
  const o = he(e) ? 1 : qr(e) ? 128 : _r(e) ? 64 : le(e) ? 4 : X(e) ? 2 : 0;
  return g(
    e,
    t,
    n,
    s,
    i,
    o,
    r,
    !0
  );
}
function Ul(e) {
  return e ? /* @__PURE__ */ js(e) || Nr(e) ? me({}, e) : e : null;
}
function _t(e, t, n = !1, s = !1) {
  const { props: i, ref: r, patchFlag: o, children: l, transition: a } = e, d = t ? Wl(i || {}, t) : i, c = {
    __v_isVNode: !0,
    __v_skip: !0,
    type: e.type,
    props: d,
    key: d && Xr(d),
    ref: t && t.ref ? (
      // #2078 in the case of <component :is="vnode" ref="extra"/>
      // if the vnode itself already has a ref, cloneVNode will need to merge
      // the refs so the single vnode can be set on multiple refs
      n && r ? U(r) ? r.concat(Pn(t)) : [r, Pn(t)] : Pn(t)
    ) : r,
    scopeId: e.scopeId,
    slotScopeIds: e.slotScopeIds,
    children: l,
    target: e.target,
    targetStart: e.targetStart,
    targetAnchor: e.targetAnchor,
    staticCount: e.staticCount,
    shapeFlag: e.shapeFlag,
    // if the vnode is cloned with extra props, we can no longer assume its
    // existing patch flag to be reliable and need to add the FULL_PROPS flag.
    // note: preserve flag for fragments since they use the flag for children
    // fast paths only.
    patchFlag: t && e.type !== te ? o === -1 ? 16 : o | 16 : o,
    dynamicProps: e.dynamicProps,
    dynamicChildren: e.dynamicChildren,
    appContext: e.appContext,
    dirs: e.dirs,
    transition: a,
    // These should technically only be non-null on mounted VNodes. However,
    // they *should* be copied for kept-alive vnodes. So we just always copy
    // them since them being non-null during a mount doesn't affect the logic as
    // they will simply be overwritten.
    component: e.component,
    suspense: e.suspense,
    ssContent: e.ssContent && _t(e.ssContent),
    ssFallback: e.ssFallback && _t(e.ssFallback),
    placeholder: e.placeholder,
    el: e.el,
    anchor: e.anchor,
    ctx: e.ctx,
    ce: e.ce
  };
  return a && s && gn(
    c,
    a.clone(c)
  ), c;
}
function Ys(e = " ", t = 0) {
  return ee(es, null, e, t);
}
function de(e = "", t = !1) {
  return t ? (S(), et(Se, null, e)) : ee(Se, null, e);
}
function Xe(e) {
  return e == null || typeof e == "boolean" ? ee(Se) : U(e) ? ee(
    te,
    null,
    // #3666, avoid reference pollution when reusing vnode
    e.slice()
  ) : Bn(e) ? at(e) : ee(es, null, String(e));
}
function at(e) {
  return e.el === null && e.patchFlag !== -1 || e.memo ? e : _t(e);
}
function Vn(e, t) {
  let n = 0;
  const { shapeFlag: s } = e;
  if (t == null)
    t = null;
  else if (U(t))
    n = 16;
  else if (typeof t == "object")
    if (s & 65) {
      const i = t.default;
      i && (i._c && (i._d = !1), Vn(e, i()), i._c && (i._d = !0));
      return;
    } else {
      n = 32;
      const i = t._;
      !i && !Nr(t) ? t._ctx = De : i === 3 && De && (De.slots._ === 1 ? t._ = 1 : (t._ = 2, e.patchFlag |= 1024));
    }
  else if (X(t)) {
    if (s & 65) {
      Vn(e, { default: t });
      return;
    }
    t = { default: t, _ctx: De }, n = 32;
  } else
    t = String(t), s & 64 ? (n = 16, t = [Ys(t)]) : n = 8;
  e.children = t, e.shapeFlag |= n;
}
function Wl(...e) {
  const t = {};
  for (let n = 0; n < e.length; n++) {
    const s = e[n];
    for (const i in s)
      if (i === "class")
        t.class !== s.class && (t.class = Ae([t.class, s.class]));
      else if (i === "style")
        t.style = Kt([t.style, s.style]);
      else if (Wn(i)) {
        const r = t[i], o = s[i];
        o && r !== o && !(U(r) && r.includes(o)) ? t[i] = r ? [].concat(r, o) : o : o == null && r == null && // mergeProps({ 'onUpdate:modelValue': undefined }) should not retain
        // the model listener.
        !qn(i) && (t[i] = o);
      } else i !== "" && (t[i] = s[i]);
  }
  return t;
}
function We(e, t, n, s = null) {
  Re(e, t, 7, [
    n,
    s
  ]);
}
const ql = Ir();
let Yl = 0;
function Xl(e, t, n) {
  const s = e.type, i = (t ? t.appContext : e.appContext) || ql, r = {
    uid: Yl++,
    vnode: e,
    type: s,
    parent: t,
    appContext: i,
    root: null,
    // to be immediately set
    next: null,
    subTree: null,
    // will be set synchronously right after creation
    effect: null,
    update: null,
    // will be set synchronously right after creation
    job: null,
    scope: new yo(
      !0
      /* detached */
    ),
    render: null,
    proxy: null,
    exposed: null,
    exposeProxy: null,
    withProxy: null,
    provides: t ? t.provides : Object.create(i.provides),
    ids: t ? t.ids : ["", 0, 0],
    accessCache: null,
    renderCache: [],
    // local resolved assets
    components: null,
    directives: null,
    // resolved props and emits options
    propsOptions: Br(s, i),
    emitsOptions: Or(s, i),
    // emit
    emit: null,
    // to be set immediately
    emitted: null,
    // props default value
    propsDefaults: ce,
    // inheritAttrs
    inheritAttrs: s.inheritAttrs,
    // state
    ctx: ce,
    data: ce,
    props: ce,
    attrs: ce,
    slots: ce,
    refs: ce,
    setupState: ce,
    setupContext: null,
    // suspense related
    suspense: n,
    suspenseId: n ? n.pendingId : 0,
    asyncDep: null,
    asyncResolved: !1,
    // lifecycle hooks
    // not using enums here because it results in computed properties
    isMounted: !1,
    isUnmounted: !1,
    isDeactivated: !1,
    bc: null,
    c: null,
    bm: null,
    m: null,
    bu: null,
    u: null,
    um: null,
    bum: null,
    da: null,
    a: null,
    rtg: null,
    rtc: null,
    ec: null,
    sp: null
  };
  return r.ctx = { _: r }, r.root = t ? t.root : r, r.emit = El.bind(null, r), e.ce && e.ce(r), r;
}
let $e = null;
const zr = () => $e || De;
let jn, As;
{
  const e = Xn(), t = (n, s) => {
    let i;
    return (i = e[n]) || (i = e[n] = []), i.push(s), (r) => {
      i.length > 1 ? i.forEach((o) => o(r)) : i[0](r);
    };
  };
  jn = t(
    "__VUE_INSTANCE_SETTERS__",
    (n) => $e = n
  ), As = t(
    "__VUE_SSR_SETTERS__",
    (n) => vn = n
  );
}
const _n = (e) => {
  const t = $e;
  return jn(e), e.scope.on(), () => {
    e.scope.off(), jn(t);
  };
}, vi = () => {
  $e && $e.scope.off(), jn(null);
};
function Gr(e) {
  return e.vnode.shapeFlag & 4;
}
let vn = !1;
function zl(e, t = !1, n = !1) {
  t && As(t);
  const { props: s, children: i } = e.vnode, r = Gr(e);
  Ll(e, s, r, t), Dl(e, i, n || t);
  const o = r ? Gl(e, t) : void 0;
  return t && As(!1), o;
}
function Gl(e, t) {
  const n = e.type;
  e.accessCache = /* @__PURE__ */ Object.create(null), e.proxy = new Proxy(e.ctx, ml);
  const { setup: s } = n;
  if (s) {
    Ze();
    const i = e.setupContext = s.length > 1 ? Zl(e) : null, r = _n(e), o = wn(
      s,
      e,
      0,
      [
        e.props,
        i
      ]
    ), l = Ki(o);
    if (Qe(), r(), (l || e.sp) && !un(e) && Ar(e), l) {
      if (o.then(vi, vi), t)
        return o.then((a) => {
          yi(e, a);
        }).catch((a) => {
          Gn(a, e, 0);
        });
      e.asyncDep = o;
    } else
      yi(e, o);
  } else
    Jr(e);
}
function yi(e, t, n) {
  X(t) ? e.type.__ssrInlineRender ? e.ssrRender = t : e.render = t : le(t) && (e.setupState = fr(t)), Jr(e);
}
function Jr(e, t, n) {
  const s = e.type;
  e.render || (e.render = s.render || Ge);
  {
    const i = _n(e);
    Ze();
    try {
      vl(e);
    } finally {
      Qe(), i();
    }
  }
}
const Jl = {
  get(e, t) {
    return xe(e, "get", ""), e[t];
  }
};
function Zl(e) {
  const t = (n) => {
    e.exposed = n || {};
  };
  return {
    attrs: new Proxy(e.attrs, Jl),
    slots: e.slots,
    emit: e.emit,
    expose: t
  };
}
function ts(e) {
  return e.exposed ? e.exposeProxy || (e.exposeProxy = new Proxy(fr(No(e.exposed)), {
    get(t, n) {
      if (n in t)
        return t[n];
      if (n in cn)
        return cn[n](e);
    },
    has(t, n) {
      return n in t || n in cn;
    }
  })) : e.proxy;
}
function Ql(e) {
  return X(e) && "__vccOpts" in e;
}
const ge = (e, t) => /* @__PURE__ */ Ko(e, t, vn);
function ea(e, t, n) {
  try {
    Hn(-1);
    const s = arguments.length;
    return s === 2 ? le(t) && !U(t) ? Bn(t) ? ee(e, null, [t]) : ee(e, t) : ee(e, null, t) : (s > 3 ? n = Array.prototype.slice.call(arguments, 2) : s === 3 && Bn(n) && (n = [n]), ee(e, t, n));
  } finally {
    Hn(1);
  }
}
const ta = "3.5.39";
let Ms;
const bi = typeof window < "u" && window.trustedTypes;
if (bi)
  try {
    Ms = /* @__PURE__ */ bi.createPolicy("vue", {
      createHTML: (e) => e
    });
  } catch {
  }
const Zr = Ms ? (e) => Ms.createHTML(e) : (e) => e, na = "http://www.w3.org/2000/svg", sa = "http://www.w3.org/1998/Math/MathML", lt = typeof document < "u" ? document : null, wi = lt && /* @__PURE__ */ lt.createElement("template"), ia = {
  insert: (e, t, n) => {
    t.insertBefore(e, n || null);
  },
  remove: (e) => {
    const t = e.parentNode;
    t && t.removeChild(e);
  },
  createElement: (e, t, n, s) => {
    const i = t === "svg" ? lt.createElementNS(na, e) : t === "mathml" ? lt.createElementNS(sa, e) : n ? lt.createElement(e, { is: n }) : lt.createElement(e);
    return e === "select" && s && s.multiple != null && i.setAttribute("multiple", s.multiple), i;
  },
  createText: (e) => lt.createTextNode(e),
  createComment: (e) => lt.createComment(e),
  setText: (e, t) => {
    e.nodeValue = t;
  },
  setElementText: (e, t) => {
    e.textContent = t;
  },
  parentNode: (e) => e.parentNode,
  nextSibling: (e) => e.nextSibling,
  querySelector: (e) => lt.querySelector(e),
  setScopeId(e, t) {
    e.setAttribute(t, "");
  },
  // __UNSAFE__
  // Reason: innerHTML.
  // Static content here can only come from compiled templates.
  // As long as the user only uses trusted templates, this is safe.
  insertStaticContent(e, t, n, s, i, r) {
    const o = n ? n.previousSibling : t.lastChild;
    if (i && (i === r || i.nextSibling))
      for (; t.insertBefore(i.cloneNode(!0), n), !(i === r || !(i = i.nextSibling)); )
        ;
    else {
      wi.innerHTML = Zr(
        s === "svg" ? `<svg>${e}</svg>` : s === "mathml" ? `<math>${e}</math>` : e
      );
      const l = wi.content;
      if (s === "svg" || s === "mathml") {
        const a = l.firstChild;
        for (; a.firstChild; )
          l.appendChild(a.firstChild);
        l.removeChild(a);
      }
      t.insertBefore(l, n);
    }
    return [
      // first
      o ? o.nextSibling : t.firstChild,
      // last
      n ? n.previousSibling : t.lastChild
    ];
  }
}, vt = "transition", Gt = "animation", yn = /* @__PURE__ */ Symbol("_vtc"), Qr = {
  name: String,
  type: String,
  css: {
    type: Boolean,
    default: !0
  },
  duration: [String, Number, Object],
  enterFromClass: String,
  enterActiveClass: String,
  enterToClass: String,
  appearFromClass: String,
  appearActiveClass: String,
  appearToClass: String,
  leaveFromClass: String,
  leaveActiveClass: String,
  leaveToClass: String
}, ra = /* @__PURE__ */ me(
  {},
  Tr,
  Qr
), oa = (e) => (e.displayName = "Transition", e.props = ra, e), xn = /* @__PURE__ */ oa(
  (e, { slots: t }) => ea(il, la(e), t)
), St = (e, t = []) => {
  U(e) ? e.forEach((n) => n(...t)) : e && e(...t);
}, _i = (e) => e ? U(e) ? e.some((t) => t.length > 1) : e.length > 1 : !1;
function la(e) {
  const t = {};
  for (const P in e)
    P in Qr || (t[P] = e[P]);
  if (e.css === !1)
    return t;
  const {
    name: n = "v",
    type: s,
    duration: i,
    enterFromClass: r = `${n}-enter-from`,
    enterActiveClass: o = `${n}-enter-active`,
    enterToClass: l = `${n}-enter-to`,
    appearFromClass: a = r,
    appearActiveClass: d = o,
    appearToClass: c = l,
    leaveFromClass: h = `${n}-leave-from`,
    leaveActiveClass: v = `${n}-leave-active`,
    leaveToClass: _ = `${n}-leave-to`
  } = e, C = aa(i), M = C && C[0], F = C && C[1], {
    onBeforeEnter: D,
    onEnter: L,
    onEnterCancelled: N,
    onLeave: T,
    onLeaveCancelled: K,
    onBeforeAppear: R = D,
    onAppear: E = L,
    onAppearCancelled: H = N
  } = t, x = (P, ie, W, J) => {
    P._enterCancelled = J, $t(P, ie ? c : l), $t(P, ie ? d : o), W && W();
  }, B = (P, ie) => {
    P._isLeaving = !1, $t(P, h), $t(P, _), $t(P, v), ie && ie();
  }, G = (P) => (ie, W) => {
    const J = P ? E : L, Z = () => x(ie, P, W);
    St(J, [ie, Z]), xi(() => {
      $t(ie, P ? a : r), ot(ie, P ? c : l), _i(J) || Ti(ie, s, M, Z);
    });
  };
  return me(t, {
    onBeforeEnter(P) {
      St(D, [P]), ot(P, r), ot(P, o);
    },
    onBeforeAppear(P) {
      St(R, [P]), ot(P, a), ot(P, d);
    },
    onEnter: G(!1),
    onAppear: G(!0),
    onLeave(P, ie) {
      P._isLeaving = !0;
      const W = () => B(P, ie);
      ot(P, h), P._enterCancelled ? (ot(P, v), Si(P)) : (Si(P), ot(P, v)), xi(() => {
        P._isLeaving && ($t(P, h), ot(P, _), _i(T) || Ti(P, s, F, W));
      }), St(T, [P, W]);
    },
    onEnterCancelled(P) {
      x(P, !1, void 0, !0), St(N, [P]);
    },
    onAppearCancelled(P) {
      x(P, !0, void 0, !0), St(H, [P]);
    },
    onLeaveCancelled(P) {
      B(P), St(K, [P]);
    }
  });
}
function aa(e) {
  if (e == null)
    return null;
  if (le(e))
    return [ds(e.enter), ds(e.leave)];
  {
    const t = ds(e);
    return [t, t];
  }
}
function ds(e) {
  return uo(e);
}
function ot(e, t) {
  t.split(/\s+/).forEach((n) => n && e.classList.add(n)), (e[yn] || (e[yn] = /* @__PURE__ */ new Set())).add(t);
}
function $t(e, t) {
  t.split(/\s+/).forEach((s) => s && e.classList.remove(s));
  const n = e[yn];
  n && (n.delete(t), n.size || (e[yn] = void 0));
}
function xi(e) {
  requestAnimationFrame(() => {
    requestAnimationFrame(e);
  });
}
let ua = 0;
function Ti(e, t, n, s) {
  const i = e._endId = ++ua, r = () => {
    i === e._endId && s();
  };
  if (n != null)
    return setTimeout(r, n);
  const { type: o, timeout: l, propCount: a } = ca(e, t);
  if (!o)
    return s();
  const d = o + "end";
  let c = 0;
  const h = () => {
    e.removeEventListener(d, v), r();
  }, v = (_) => {
    _.target === e && ++c >= a && h();
  };
  setTimeout(() => {
    c < a && h();
  }, l + 1), e.addEventListener(d, v);
}
function ca(e, t) {
  const n = window.getComputedStyle(e), s = (C) => (n[C] || "").split(", "), i = s(`${vt}Delay`), r = s(`${vt}Duration`), o = Ci(i, r), l = s(`${Gt}Delay`), a = s(`${Gt}Duration`), d = Ci(l, a);
  let c = null, h = 0, v = 0;
  t === vt ? o > 0 && (c = vt, h = o, v = r.length) : t === Gt ? d > 0 && (c = Gt, h = d, v = a.length) : (h = Math.max(o, d), c = h > 0 ? o > d ? vt : Gt : null, v = c ? c === vt ? r.length : a.length : 0);
  const _ = c === vt && /\b(?:transform|all)(?:,|$)/.test(
    s(`${vt}Property`).toString()
  );
  return {
    type: c,
    timeout: h,
    propCount: v,
    hasTransform: _
  };
}
function Ci(e, t) {
  for (; e.length < t.length; )
    e = e.concat(e);
  return Math.max(...t.map((n, s) => Ei(n) + Ei(e[s])));
}
function Ei(e) {
  return e === "auto" ? 0 : Number(e.slice(0, -1).replace(",", ".")) * 1e3;
}
function Si(e) {
  return (e ? e.ownerDocument : document).body.offsetHeight;
}
function fa(e, t, n) {
  const s = e[yn];
  s && (t = (t ? [t, ...s] : [...s]).join(" ")), t == null ? e.removeAttribute("class") : n ? e.setAttribute("class", t) : e.className = t;
}
const Kn = /* @__PURE__ */ Symbol("_vod"), eo = /* @__PURE__ */ Symbol("_vsh"), da = {
  // used for prop mismatch check during hydration
  name: "show",
  beforeMount(e, { value: t }, { transition: n }) {
    e[Kn] = e.style.display === "none" ? "" : e.style.display, n && t ? n.beforeEnter(e) : Jt(e, t);
  },
  mounted(e, { value: t }, { transition: n }) {
    n && t && n.enter(e);
  },
  updated(e, { value: t, oldValue: n }, { transition: s }) {
    !t != !n && (s ? t ? (s.beforeEnter(e), Jt(e, !0), s.enter(e)) : s.leave(e, () => {
      Jt(e, !1);
    }) : Jt(e, t));
  },
  beforeUnmount(e, { value: t }) {
    Jt(e, t);
  }
};
function Jt(e, t) {
  e.style.display = t ? e[Kn] : "none", e[eo] = !t;
}
const ha = /* @__PURE__ */ Symbol(""), pa = /(?:^|;)\s*display\s*:/;
function ga(e, t, n) {
  const s = e.style, i = he(n);
  let r = !1;
  if (n && !i) {
    if (t)
      if (he(t))
        for (const o of t.split(";")) {
          const l = o.slice(0, o.indexOf(":")).trim();
          n[l] == null && en(s, l, "");
        }
      else
        for (const o in t)
          n[o] == null && en(s, o, "");
    for (const o in n) {
      o === "display" && (r = !0);
      const l = n[o];
      l != null ? va(
        e,
        o,
        !he(t) && t ? t[o] : void 0,
        l
      ) || en(s, o, l) : en(s, o, "");
    }
  } else if (i) {
    if (t !== n) {
      const o = s[ha];
      o && (n += ";" + o), s.cssText = n, r = pa.test(n);
    }
  } else t && e.removeAttribute("style");
  Kn in e && (e[Kn] = r ? s.display : "", e[eo] && (s.display = "none"));
}
const $i = /\s*!important$/;
function en(e, t, n) {
  if (U(n))
    n.forEach((s) => en(e, t, s));
  else if (n == null && (n = ""), t.startsWith("--"))
    e.setProperty(t, n);
  else {
    const s = ma(e, t);
    $i.test(n) ? e.setProperty(
      It(s),
      n.replace($i, ""),
      "important"
    ) : e[s] = n;
  }
}
const Ai = ["Webkit", "Moz", "ms"], hs = {};
function ma(e, t) {
  const n = hs[t];
  if (n)
    return n;
  let s = He(t);
  if (s !== "filter" && s in e)
    return hs[t] = s;
  s = qi(s);
  for (let i = 0; i < Ai.length; i++) {
    const r = Ai[i] + s;
    if (r in e)
      return hs[t] = r;
  }
  return t;
}
function va(e, t, n, s) {
  return e.tagName === "TEXTAREA" && (t === "width" || t === "height") && he(s) && n === s;
}
const Mi = "http://www.w3.org/1999/xlink";
function ki(e, t, n, s, i, r = mo(t)) {
  s && t.startsWith("xlink:") ? n == null ? e.removeAttributeNS(Mi, t.slice(6, t.length)) : e.setAttributeNS(Mi, t, n) : n == null || r && !Xi(n) ? e.removeAttribute(t) : e.setAttribute(
    t,
    r ? "" : Je(n) ? String(n) : n
  );
}
function Li(e, t, n, s, i) {
  if (t === "innerHTML" || t === "textContent") {
    n != null && (e[t] = t === "innerHTML" ? Zr(n) : n);
    return;
  }
  const r = e.tagName;
  if (t === "value" && r !== "PROGRESS" && // custom elements may use _value internally
  !r.includes("-")) {
    const l = r === "OPTION" ? e.getAttribute("value") || "" : e.value, a = n == null ? (
      // #11647: value should be set as empty string for null and undefined,
      // but <input type="checkbox"> should be set as 'on'.
      e.type === "checkbox" ? "on" : ""
    ) : String(n);
    (l !== a || !("_value" in e)) && (e.value = a), n == null && e.removeAttribute(t), e._value = n;
    return;
  }
  let o = !1;
  if (n === "" || n == null) {
    const l = typeof e[t];
    l === "boolean" ? n = Xi(n) : n == null && l === "string" ? (n = "", o = !0) : l === "number" && (n = 0, o = !0);
  }
  try {
    e[t] = n;
  } catch {
  }
  o && e.removeAttribute(i || t);
}
function Rt(e, t, n, s) {
  e.addEventListener(t, n, s);
}
function ya(e, t, n, s) {
  e.removeEventListener(t, n, s);
}
const Pi = /* @__PURE__ */ Symbol("_vei");
function ba(e, t, n, s, i = null) {
  const r = e[Pi] || (e[Pi] = {}), o = r[t];
  if (s && o)
    o.value = s;
  else {
    const [l, a] = xa(t);
    if (s) {
      const d = r[t] = Ea(
        s,
        i
      );
      Rt(e, l, d, a);
    } else o && (ya(e, l, o, a), r[t] = void 0);
  }
}
const wa = /(Once|Passive|Capture)$/, _a = /^on:?(?:Once|Passive|Capture)$/;
function xa(e) {
  let t, n;
  for (; (n = e.match(wa)) && !_a.test(e); )
    t || (t = {}), e = e.slice(0, e.length - n[1].length), t[n[1].toLowerCase()] = !0;
  return [e[2] === ":" ? e.slice(3) : It(e.slice(2)), t];
}
let ps = 0;
const Ta = /* @__PURE__ */ Promise.resolve(), Ca = () => ps || (Ta.then(() => ps = 0), ps = Date.now());
function Ea(e, t) {
  const n = (s) => {
    if (!s._vts)
      s._vts = Date.now();
    else if (s._vts <= n.attached)
      return;
    const i = n.value;
    if (U(i)) {
      const r = s.stopImmediatePropagation;
      s.stopImmediatePropagation = () => {
        r.call(s), s._stopped = !0;
      };
      const o = i.slice(), l = [s];
      for (let a = 0; a < o.length && !s._stopped; a++) {
        const d = o[a];
        d && Re(
          d,
          t,
          5,
          l
        );
      }
    } else
      Re(
        i,
        t,
        5,
        [s]
      );
  };
  return n.value = e, n.attached = Ca(), n;
}
const Ii = (e) => e.charCodeAt(0) === 111 && e.charCodeAt(1) === 110 && // lowercase letter
e.charCodeAt(2) > 96 && e.charCodeAt(2) < 123, Sa = (e, t, n, s, i, r) => {
  const o = i === "svg";
  t === "class" ? fa(e, s, o) : t === "style" ? ga(e, n, s) : Wn(t) ? qn(t) || ba(e, t, n, s, r) : (t[0] === "." ? (t = t.slice(1), !0) : t[0] === "^" ? (t = t.slice(1), !1) : $a(e, t, s, o)) ? (Li(e, t, s), !e.tagName.includes("-") && (t === "value" || t === "checked" || t === "selected") && ki(e, t, s, o, r, t !== "value")) : /* #11081 force set props for possible async custom element */ e._isVueCE && // #12408 check if it's declared prop or it's async custom element
  (Aa(e, t) || // @ts-expect-error _def is private
  e._def.__asyncLoader && (/[A-Z]/.test(t) || !he(s))) ? Li(e, He(t), s, r, t) : (t === "true-value" ? e._trueValue = s : t === "false-value" && (e._falseValue = s), ki(e, t, s, o));
};
function $a(e, t, n, s) {
  if (s)
    return !!(t === "innerHTML" || t === "textContent" || t in e && Ii(t) && X(n));
  if (t === "spellcheck" || t === "draggable" || t === "translate" || t === "autocorrect" || t === "sandbox" && e.tagName === "IFRAME" || t === "form" || t === "list" && e.tagName === "INPUT" || t === "type" && e.tagName === "TEXTAREA")
    return !1;
  if (t === "width" || t === "height") {
    const i = e.tagName;
    if (i === "IMG" || i === "VIDEO" || i === "CANVAS" || i === "SOURCE")
      return !1;
  }
  return Ii(t) && he(n) ? !1 : t in e;
}
function Aa(e, t) {
  const n = (
    // @ts-expect-error _def is private
    e._def.props
  );
  if (!n)
    return !1;
  const s = He(t);
  return Array.isArray(n) ? n.some((i) => He(i) === s) : Object.keys(n).some((i) => He(i) === s);
}
const Oi = (e) => {
  const t = e.props["onUpdate:modelValue"] || !1;
  return U(t) ? (n) => kn(t, n) : t;
};
function Ma(e) {
  e.target.composing = !0;
}
function Di(e) {
  const t = e.target;
  t.composing && (t.composing = !1, t.dispatchEvent(new Event("input")));
}
const gs = /* @__PURE__ */ Symbol("_assign");
function Fi(e, t, n) {
  return t && (e = e.trim()), n && (e = Is(e)), e;
}
const ka = {
  created(e, { modifiers: { lazy: t, trim: n, number: s } }, i) {
    e[gs] = Oi(i);
    const r = s || i.props && i.props.type === "number";
    Rt(e, t ? "change" : "input", (o) => {
      o.target.composing || e[gs](Fi(e.value, n, r));
    }), (n || r) && Rt(e, "change", () => {
      e.value = Fi(e.value, n, r);
    }), t || (Rt(e, "compositionstart", Ma), Rt(e, "compositionend", Di), Rt(e, "change", Di));
  },
  // set value on mounted so it's after min/max for type="range"
  mounted(e, { value: t }) {
    e.value = t ?? "";
  },
  beforeUpdate(e, { value: t, oldValue: n, modifiers: { lazy: s, trim: i, number: r } }, o) {
    if (e[gs] = Oi(o), e.composing) return;
    const l = (r || e.type === "number") && !/^0\d/.test(e.value) ? Is(e.value) : e.value, a = t ?? "";
    if (l === a)
      return;
    const d = e.getRootNode();
    (d instanceof Document || d instanceof ShadowRoot) && d.activeElement === e && e.type !== "range" && (s && t === n || i && e.value.trim() === a) || (e.value = a);
  }
}, La = ["ctrl", "shift", "alt", "meta"], Pa = {
  stop: (e) => e.stopPropagation(),
  prevent: (e) => e.preventDefault(),
  self: (e) => e.target !== e.currentTarget,
  ctrl: (e) => !e.ctrlKey,
  shift: (e) => !e.shiftKey,
  alt: (e) => !e.altKey,
  meta: (e) => !e.metaKey,
  left: (e) => "button" in e && e.button !== 0,
  middle: (e) => "button" in e && e.button !== 1,
  right: (e) => "button" in e && e.button !== 2,
  exact: (e, t) => La.some((n) => e[`${n}Key`] && !t.includes(n))
}, Le = (e, t) => {
  if (!e) return e;
  const n = e._withMods || (e._withMods = {}), s = t.join(".");
  return n[s] || (n[s] = ((i, ...r) => {
    for (let o = 0; o < t.length; o++) {
      const l = Pa[t[o]];
      if (l && l(i, t)) return;
    }
    return e(i, ...r);
  }));
}, Ia = /* @__PURE__ */ me({ patchProp: Sa }, ia);
let Ri;
function Oa() {
  return Ri || (Ri = Rl(Ia));
}
const Da = ((...e) => {
  const t = Oa().createApp(...e), { mount: n } = t;
  return t.mount = (s) => {
    const i = Ra(s);
    if (!i) return;
    const r = t._component;
    !X(r) && !r.render && !r.template && (r.template = i.innerHTML), i.nodeType === 1 && (i.textContent = "");
    const o = n(i, !1, Fa(i));
    return i instanceof Element && (i.removeAttribute("v-cloak"), i.setAttribute("data-v-app", "")), o;
  }, t;
});
function Fa(e) {
  if (e instanceof SVGElement)
    return "svg";
  if (typeof MathMLElement == "function" && e instanceof MathMLElement)
    return "mathml";
}
function Ra(e) {
  return he(e) ? document.querySelector(e) : e;
}
const Na = document.querySelector("#app"), Ha = (Na?.dataset.basePath || "/drop").replace(/\/$/, "");
function Wt(e) {
  return `${Ha}${e.startsWith("/") ? e : `/${e}`}`;
}
async function to(e) {
  try {
    const t = await e.json();
    return new Error(t.error?.message || `请求失败 (${e.status})`);
  } catch {
    return new Error(`请求失败 (${e.status})`);
  }
}
async function Xs(e, t) {
  const n = await fetch(e, {
    ...t,
    headers: { Accept: "application/json", ...t?.headers }
  });
  if (n.status === 401)
    throw location.reload(), new Error("登录状态已失效");
  if (!n.ok) throw await to(n);
  return n.json();
}
async function Ba(e) {
  return (await Xs(Wt("/api/v1/items?limit=100"), { signal: e })).items || [];
}
function Va(e) {
  return fetch(e);
}
async function ja(e, t) {
  await Xs(Wt(`/api/v1/items/${encodeURIComponent(e)}/expiry`), {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ ttl_days: t })
  });
}
async function Ka(e) {
  const t = await fetch(Wt(`/api/v1/items/${encodeURIComponent(e)}`), { method: "DELETE" });
  if (t.status === 401)
    throw location.reload(), new Error("登录状态已失效");
  if (!t.ok) throw await to(t);
}
function Ua() {
  return Xs(Wt("/api/v1/status"));
}
const Wa = {
  viewBox: "0 0 24 24",
  "aria-hidden": "true",
  fill: "none",
  stroke: "currentColor",
  "stroke-width": "1.8",
  "stroke-linecap": "round",
  "stroke-linejoin": "round"
}, qa = {
  key: 0,
  d: "M12 5v14M5 12h14"
}, Ya = {
  key: 2,
  d: "M12 19V5m-6 6 6-6 6 6"
}, Xa = {
  key: 3,
  x: "7",
  y: "7",
  width: "10",
  height: "10",
  rx: "2",
  fill: "currentColor",
  stroke: "none"
}, za = {
  key: 4,
  d: "m14 6-6 6 6 6"
}, Ga = {
  key: 9,
  d: "m6 6 12 12M18 6 6 18"
}, Ja = {
  key: 13,
  d: "m5 12 4 4L19 6"
}, ve = /* @__PURE__ */ tt({
  __name: "AppIcon",
  props: {
    name: {}
  },
  setup(e) {
    return (t, n) => (S(), I("svg", Wa, [
      e.name === "plus" ? (S(), I("path", qa)) : e.name === "settings" ? (S(), I(te, { key: 1 }, [
        n[0] || (n[0] = g("path", { d: "M4 7h5m4 0h7M4 17h8m4 0h4" }, null, -1)),
        n[1] || (n[1] = g("circle", {
          cx: "11",
          cy: "7",
          r: "2"
        }, null, -1)),
        n[2] || (n[2] = g("circle", {
          cx: "14",
          cy: "17",
          r: "2"
        }, null, -1))
      ], 64)) : e.name === "send" ? (S(), I("path", Ya)) : e.name === "stop" ? (S(), I("rect", Xa)) : e.name === "back" ? (S(), I("path", za)) : e.name === "refresh" ? (S(), I(te, { key: 5 }, [
        n[3] || (n[3] = g("path", { d: "M20 11a8 8 0 1 0-2.34 5.66" }, null, -1)),
        n[4] || (n[4] = g("path", { d: "M20 4v7h-7" }, null, -1))
      ], 64)) : e.name === "more" ? (S(), I(te, { key: 6 }, [
        n[5] || (n[5] = g("circle", {
          cx: "5",
          cy: "12",
          r: ".8",
          fill: "currentColor",
          stroke: "none"
        }, null, -1)),
        n[6] || (n[6] = g("circle", {
          cx: "12",
          cy: "12",
          r: ".8",
          fill: "currentColor",
          stroke: "none"
        }, null, -1)),
        n[7] || (n[7] = g("circle", {
          cx: "19",
          cy: "12",
          r: ".8",
          fill: "currentColor",
          stroke: "none"
        }, null, -1))
      ], 64)) : e.name === "copy" ? (S(), I(te, { key: 7 }, [
        n[8] || (n[8] = g("rect", {
          x: "8",
          y: "8",
          width: "11",
          height: "11",
          rx: "2"
        }, null, -1)),
        n[9] || (n[9] = g("path", { d: "M16 5V4a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h1" }, null, -1))
      ], 64)) : e.name === "download" ? (S(), I(te, { key: 8 }, [
        n[10] || (n[10] = g("path", { d: "M12 3v12m0 0 4-4m-4 4-4-4" }, null, -1)),
        n[11] || (n[11] = g("path", { d: "M4 18v2h16v-2" }, null, -1))
      ], 64)) : e.name === "close" ? (S(), I("path", Ga)) : e.name === "trash" ? (S(), I(te, { key: 10 }, [
        n[12] || (n[12] = g("path", { d: "M4 7h16M9 7V4h6v3m3 0-1 14H7L6 7" }, null, -1)),
        n[13] || (n[13] = g("path", { d: "M10 11v6m4-6v6" }, null, -1))
      ], 64)) : e.name === "clock" ? (S(), I(te, { key: 11 }, [
        n[14] || (n[14] = g("circle", {
          cx: "12",
          cy: "12",
          r: "9"
        }, null, -1)),
        n[15] || (n[15] = g("path", { d: "M12 7v5l3 2" }, null, -1))
      ], 64)) : e.name === "device" ? (S(), I(te, { key: 12 }, [
        n[16] || (n[16] = g("rect", {
          x: "6",
          y: "3",
          width: "12",
          height: "18",
          rx: "2"
        }, null, -1)),
        n[17] || (n[17] = g("path", { d: "M10 17h4" }, null, -1))
      ], 64)) : e.name === "check" ? (S(), I("path", Ja)) : (S(), I(te, { key: 14 }, [
        n[18] || (n[18] = g("path", { d: "M7 3h7l4 4v14H7z" }, null, -1)),
        n[19] || (n[19] = g("path", { d: "M14 3v5h5" }, null, -1))
      ], 64))
    ]));
  }
}), no = [1, 3, 7];
function wt(e) {
  if (!Number.isFinite(e) || e <= 0) return "0 B";
  const t = ["B", "KB", "MB", "GB"], n = Math.min(Math.floor(Math.log(e) / Math.log(1024)), t.length - 1), s = e / 1024 ** n;
  return `${s >= 10 || n === 0 ? s.toFixed(0) : s.toFixed(1)} ${t[n]}`;
}
function Za(e) {
  return new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
    hour12: !1
  }).format(new Date(e));
}
function tn(e) {
  const t = e instanceof Date ? e : new Date(e);
  return `${t.getFullYear()}-${t.getMonth()}-${t.getDate()}`;
}
function Qa(e) {
  const t = new Date(e), n = /* @__PURE__ */ new Date(), s = new Date(n);
  return s.setDate(n.getDate() - 1), tn(t) === tn(n) ? "今天" : tn(t) === tn(s) ? "昨天" : new Intl.DateTimeFormat("zh-CN", { month: "long", day: "numeric" }).format(t);
}
function Ni(e) {
  const t = new Date(e).getTime() - Date.now();
  if (t <= 0) return "即将过期";
  const n = Math.ceil(t / 36e5);
  return n < 24 ? `${n} 小时后过期` : `${Math.ceil(n / 24)} 天后过期`;
}
function Un(e) {
  return e === 1 ? "24 小时" : `${e} 天`;
}
function eu(e) {
  if (!Number.isFinite(e) || e <= 1) return "不到 1 秒";
  if (e < 60) return `约 ${Math.ceil(e)} 秒`;
  const t = Math.ceil(e / 60);
  if (t < 60) return `约 ${t} 分钟`;
  const n = Math.floor(t / 60), s = t % 60;
  return s ? `约 ${n} 小时 ${s} 分钟` : `约 ${n} 小时`;
}
function so(e) {
  return ((e.includes(".") ? e.split(".").pop() : "FILE") || "FILE").slice(0, 4).toUpperCase();
}
function tu(e) {
  const t = /https?:\/\/[^\s]+/g, n = [];
  let s = 0;
  for (const i of e.matchAll(t)) {
    const r = i.index ?? 0;
    r > s && n.push({ type: "text", value: e.slice(s, r) }), n.push({ type: "link", value: i[0] }), s = r + i[0].length;
  }
  return s < e.length && n.push({ type: "text", value: e.slice(s) }), n;
}
function nn(e) {
  return e instanceof Error ? e.message : "操作失败，请重试";
}
function io() {
  try {
    const e = JSON.parse(sessionStorage.getItem("drop.preview-history.v1") || "[]");
    return new Set(Array.isArray(e) ? e.filter((t) => typeof t == "string").slice(-200) : []);
  } catch {
    return /* @__PURE__ */ new Set();
  }
}
function nu(e) {
  try {
    const t = io();
    t.add(e), sessionStorage.setItem("drop.preview-history.v1", JSON.stringify(Array.from(t).slice(-200)));
  } catch {
  }
}
const su = ["aria-expanded"], iu = { class: "settings-heading" }, ru = { class: "ttl-list" }, ou = ["onClick"], lu = { class: "status-row" }, au = { class: "storage-track" }, uu = { class: "status-row" }, cu = { key: 1 }, fu = /* @__PURE__ */ tt({
  __name: "SettingsPopover",
  props: {
    modelValue: {}
  },
  emits: ["update:modelValue", "toast"],
  setup(e, { emit: t }) {
    const n = t, s = /* @__PURE__ */ z(null), i = /* @__PURE__ */ z(!1), r = /* @__PURE__ */ z(!1), o = /* @__PURE__ */ z(null);
    async function l() {
      if (i.value = !i.value, i.value && !o.value) {
        r.value = !0;
        try {
          o.value = await Ua();
        } catch (c) {
          n("toast", nn(c));
        } finally {
          r.value = !1;
        }
      }
    }
    function a(c) {
      n("update:modelValue", c), n("toast", `新消息保留 ${Un(c)}`), i.value = !1;
    }
    function d(c) {
      i.value && s.value && !s.value.contains(c.target) && (i.value = !1);
    }
    return gt(() => document.addEventListener("pointerdown", d)), xt(() => document.removeEventListener("pointerdown", d)), (c, h) => (S(), I("div", {
      ref_key: "root",
      ref: s,
      class: "settings-popover"
    }, [
      g("button", {
        class: "composer-icon-button",
        type: "button",
        "aria-label": "消息设置",
        "aria-expanded": i.value,
        onClick: l
      }, [
        ee(ve, { name: "settings" })
      ], 8, su),
      ee(xn, { name: "menu-pop" }, {
        default: Ut(() => [
          i.value ? (S(), I("section", {
            key: 0,
            class: "settings-panel",
            "aria-label": "消息设置",
            onPointerdown: h[1] || (h[1] = Le(() => {
            }, ["stop"]))
          }, [
            g("div", iu, [
              h[2] || (h[2] = g("div", null, [
                g("p", { class: "menu-caption" }, "设置"),
                g("h2", null, "保存期限")
              ], -1)),
              g("button", {
                class: "panel-close",
                type: "button",
                "aria-label": "关闭设置",
                onClick: h[0] || (h[0] = (v) => i.value = !1)
              }, [
                ee(ve, { name: "close" })
              ])
            ]),
            g("div", ru, [
              (S(!0), I(te, null, ft(we(no), (v) => (S(), I("button", {
                key: v,
                type: "button",
                class: Ae({ selected: e.modelValue === v }),
                onClick: (_) => a(v)
              }, [
                g("span", null, ne(we(Un)(v)), 1),
                e.modelValue === v ? (S(), et(ve, {
                  key: 0,
                  name: "check"
                })) : de("", !0)
              ], 10, ou))), 128))
            ]),
            g("div", {
              class: Ae(["status-block", { loading: r.value }])
            }, [
              o.value ? (S(), I(te, { key: 0 }, [
                g("div", lu, [
                  h[3] || (h[3] = g("span", null, "存储空间", -1)),
                  g("strong", null, ne(we(wt)(o.value.storage.used_bytes)) + " / " + ne(we(wt)(o.value.storage.quota_bytes)), 1)
                ]),
                g("div", au, [
                  g("i", {
                    style: Kt({ width: `${Math.min(100, o.value.storage.used_bytes / o.value.storage.quota_bytes * 100)}%` })
                  }, null, 4)
                ]),
                g("div", uu, [
                  h[4] || (h[4] = g("span", null, "近 24 小时流量", -1)),
                  g("strong", null, ne(we(wt)(o.value.traffic.last_24_hours.total_bytes)), 1)
                ]),
                h[5] || (h[5] = g("p", null, "登录和分享权限由 HomeHub 统一管理", -1))
              ], 64)) : (S(), I("span", cu, ne(r.value ? "正在读取状态…" : "暂时无法读取状态"), 1))
            ], 2)
          ], 32)) : de("", !0)
        ]),
        _: 1
      })
    ], 512));
  }
}), du = { class: "composer-dock" }, hu = {
  key: 0,
  class: "page-drop-overlay",
  "aria-hidden": "true"
}, pu = { class: "page-drop-prompt" }, gu = ["disabled"], mu = {
  key: 0,
  class: "selected-files",
  "aria-label": "待上传文件"
}, vu = { class: "selected-file-type" }, yu = { class: "selected-file-copy" }, bu = ["title"], wu = ["disabled", "aria-label", "onClick"], _u = {
  key: 1,
  class: "upload-progress",
  "aria-live": "polite"
}, xu = { class: "upload-progress-copy" }, Tu = { class: "upload-track" }, Cu = { class: "composer-footer" }, Eu = { class: "composer-tools" }, Su = ["disabled"], $u = {
  key: 1,
  class: "file-total"
}, Au = { class: "composer-actions" }, Mu = ["aria-label", "title"], ku = ["disabled", "aria-label", "title"], Lu = {
  key: 2,
  class: "composer-error",
  role: "alert"
}, Pu = 10, Iu = 500 * 1024 * 1024, Hi = 1024 * 1024 * 1024, Ou = 50 * 1024 * 1024, Du = /* @__PURE__ */ tt({
  __name: "ComposerBox",
  props: {
    owner: { type: Boolean },
    connectionState: {}
  },
  emits: ["sent", "toast"],
  setup(e, { emit: t }) {
    const n = e, s = t, i = /* @__PURE__ */ z(""), r = /* @__PURE__ */ z([]), o = /* @__PURE__ */ z(1), l = /* @__PURE__ */ z(null), a = /* @__PURE__ */ z(null), d = /* @__PURE__ */ z(!1), c = /* @__PURE__ */ z("idle"), h = /* @__PURE__ */ z(0), v = /* @__PURE__ */ z(0), _ = /* @__PURE__ */ z(0), C = /* @__PURE__ */ z("");
    let M = null, F = 0, D = !1, L = 0, N = 0, T = 0, K = 0;
    const R = ge(() => c.value !== "idle"), E = ge(() => c.value === "preparing" || c.value === "uploading"), H = ge(() => c.value === "processing" ? "check" : R.value ? "stop" : "send"), x = ge(() => r.value.reduce((u, f) => u + f.size, 0)), B = ge(() => c.value === "preparing" ? "正在准备文件" : c.value === "uploading" ? `正在上传 · ${h.value}%` : c.value === "processing" ? "上传完成 · 服务器正在保存" : ""), G = ge(() => c.value === "preparing" ? "等待上传" : c.value === "processing" ? "请稍候" : v.value <= 0 ? "正在估算速度" : `${wt(v.value)}/s · ${eu(_.value)}`), P = ge(() => ({
      connected: "实时连接正常",
      connecting: "正在建立实时连接",
      disconnected: "实时连接已断开，正在重试",
      offline: "设备当前没有网络"
    })[n.connectionState]);
    function ie(u) {
      return Array.from(u.dataTransfer?.types || []).includes("Files");
    }
    function W(u) {
      !ie(u) || R.value || (u.preventDefault(), F += 1, d.value = !0);
    }
    function J(u) {
      !ie(u) || R.value || (u.preventDefault(), u.dataTransfer && (u.dataTransfer.dropEffect = "copy"), d.value = !0);
    }
    function Z(u) {
      d.value && (u.preventDefault(), F = Math.max(0, F - 1), F === 0 && (d.value = !1));
    }
    function Y() {
      F = 0, d.value = !1;
    }
    function Q() {
      R.value || l.value?.click();
    }
    function Ne(u) {
      const f = Array.from(u);
      if (!f.length) return;
      C.value = "";
      const p = [...r.value];
      for (const m of f) {
        if (p.length >= Pu) {
          C.value = "一条消息最多添加 10 个文件";
          break;
        }
        if (m.size > Iu) {
          C.value = `${m.name} 超过单文件 500 MB 限制`;
          continue;
        }
        if (!p.some((y) => y.name === m.name && y.size === m.size && y.lastModified === m.lastModified)) {
          if (p.reduce((y, k) => y + k.size, 0) + m.size > Hi) {
            C.value = "本次文件总量超过 1 GB 限制";
            break;
          }
          p.push(m);
        }
      }
      r.value = p;
    }
    function Ot() {
      l.value?.files && Ne(l.value.files), l.value && (l.value.value = "");
    }
    function nt(u) {
      const f = u.clipboardData?.files;
      f?.length && Ne(f);
    }
    function Me(u) {
      u.preventDefault(), Y(), !R.value && u.dataTransfer?.files && Ne(u.dataTransfer.files);
    }
    function Tn(u) {
      R.value || r.value.splice(u, 1);
    }
    function Cn() {
      const u = a.value;
      u && (u.style.height = "auto", u.style.height = `${Math.min(u.scrollHeight, 180)}px`);
    }
    function ns(u) {
      u.key === "Enter" && !u.shiftKey && !u.isComposing && (u.preventDefault(), it());
    }
    function Tt() {
      i.value = "", r.value = [], h.value = 0, C.value = "", ht(Cn);
    }
    function mt() {
      v.value = 0, _.value = 0, N = 0, T = 0, K = 0;
    }
    function st() {
      c.value = "idle", M = null;
    }
    function qt(u, f, p = 0) {
      const m = new XMLHttpRequest();
      M = m, m.open("POST", Wt("/api/v1/items")), m.responseType = "json", m.setRequestHeader("Idempotency-Key", f), m.upload.addEventListener("loadstart", () => {
        mt(), N = performance.now(), T = N, c.value = "uploading";
      }), m.upload.addEventListener("progress", (b) => {
        if (c.value = "uploading", !b.lengthComputable) return;
        h.value = Math.min(100, Math.round(b.loaded / b.total * 100));
        const y = performance.now(), k = Math.max(1e-3, (y - N) / 1e3), A = Math.max(1e-3, (y - T) / 1e3), $ = b.loaded / k, w = (b.loaded - K) / A, V = K > 0 && A >= 0.08 ? w : $;
        V > 0 && Number.isFinite(V) && (v.value = v.value > 0 ? v.value * 0.72 + V * 0.28 : V, _.value = Math.max(0, (b.total - b.loaded) / v.value)), T = y, K = b.loaded;
      }), m.upload.addEventListener("load", () => {
        h.value = 100, c.value = "processing";
      }), m.addEventListener("load", () => {
        if (m.status >= 200 && m.status < 300) {
          Tt(), st(), s("toast", "已发送"), s("sent");
          return;
        }
        C.value = m.response?.error?.message || `发送失败 (${m.status})，请重试`, st(), m.status === 401 && location.reload();
      }), m.addEventListener("error", () => {
        if (p === 0 && !D && navigator.onLine) {
          M = null, c.value = "preparing", h.value = 0, mt(), C.value = "连接波动，正在自动重试一次…", L = window.setTimeout(() => qt(u, f, 1), 700);
          return;
        }
        C.value = "网络连接中断，文件仍保留在发送区，可再次发送", st();
      }), m.addEventListener("abort", () => {
        D && (C.value = "已取消上传，文件仍保留在发送区", st());
      }), m.send(u);
    }
    async function it() {
      if (R.value) return;
      const u = new TextEncoder().encode(i.value).byteLength;
      if (!i.value.length && !r.value.length) {
        C.value = "输入文字，或者添加一个文件", a.value?.focus();
        return;
      }
      if (u > Ou) {
        C.value = "文字内容超过 50 MB 限制";
        return;
      }
      if (u + x.value > Hi) {
        C.value = "本次内容总量超过 1 GB 限制";
        return;
      }
      const f = new FormData();
      i.value.length && f.append("text", i.value), n.owner && f.append("ttl_days", String(o.value));
      for (const p of r.value) f.append("files", p, p.name);
      C.value = "", h.value = 0, mt(), c.value = "preparing", D = !1, await ht(), qt(f, crypto.randomUUID());
    }
    function zs() {
      D = !0, window.clearTimeout(L), M?.abort(), M || st();
    }
    return gt(() => {
      window.addEventListener("dragenter", W), window.addEventListener("dragover", J), window.addEventListener("dragleave", Z), window.addEventListener("drop", Me), window.addEventListener("dragend", Y), window.addEventListener("blur", Y);
    }), xt(() => {
      window.clearTimeout(L), M?.abort(), window.removeEventListener("dragenter", W), window.removeEventListener("dragover", J), window.removeEventListener("dragleave", Z), window.removeEventListener("drop", Me), window.removeEventListener("dragend", Y), window.removeEventListener("blur", Y);
    }), (u, f) => (S(), I("div", du, [
      g("form", {
        class: Ae(["composer-box", { "is-dragging": d.value, "is-busy": R.value }]),
        "aria-label": "发送消息",
        onSubmit: Le(it, ["prevent"])
      }, [
        (S(), et(xr, { to: "body" }, [
          ee(xn, { name: "drag-overlay" }, {
            default: Ut(() => [
              d.value ? (S(), I("div", hu, [
                g("div", pu, [
                  ee(ve, { name: "plus" }),
                  f[4] || (f[4] = g("strong", null, "松开以添加文件", -1)),
                  f[5] || (f[5] = g("span", null, "可放到页面任意位置", -1))
                ])
              ])) : de("", !0)
            ]),
            _: 1
          })
        ])),
        f[6] || (f[6] = g("label", {
          class: "sr-only",
          for: "message-input"
        }, "消息内容", -1)),
        vr(g("textarea", {
          id: "message-input",
          ref_key: "textarea",
          ref: a,
          "onUpdate:modelValue": f[0] || (f[0] = (p) => i.value = p),
          rows: "1",
          maxlength: "52428800",
          placeholder: "粘贴文字、网址或截图…",
          disabled: R.value,
          onInput: Cn,
          onKeydown: ns,
          onPaste: nt
        }, null, 40, gu), [
          [ka, i.value]
        ]),
        r.value.length ? (S(), I("div", mu, [
          (S(!0), I(te, null, ft(r.value, (p, m) => (S(), I("div", {
            key: `${p.name}-${p.lastModified}-${m}`,
            class: "selected-file"
          }, [
            g("span", vu, ne(we(so)(p.name)), 1),
            g("span", yu, [
              g("strong", {
                title: p.name
              }, ne(p.name), 9, bu),
              g("small", null, ne(we(wt)(p.size)), 1)
            ]),
            g("button", {
              type: "button",
              disabled: R.value,
              "aria-label": `移除 ${p.name}`,
              onClick: (b) => Tn(m)
            }, [
              ee(ve, { name: "close" })
            ], 8, wu)
          ]))), 128))
        ])) : de("", !0),
        R.value ? (S(), I("div", _u, [
          g("div", xu, [
            g("strong", null, ne(B.value), 1),
            g("span", null, ne(G.value), 1)
          ]),
          g("div", Tu, [
            g("i", {
              style: Kt({ width: `${c.value === "preparing" ? 2 : h.value}%` })
            }, null, 4)
          ])
        ])) : de("", !0),
        g("div", Cu, [
          g("div", Eu, [
            g("input", {
              ref_key: "fileInput",
              ref: l,
              type: "file",
              multiple: "",
              hidden: "",
              onChange: Ot
            }, null, 544),
            g("button", {
              class: "composer-icon-button",
              type: "button",
              disabled: R.value,
              "aria-label": "添加文件",
              onClick: Q
            }, [
              ee(ve, { name: "plus" })
            ], 8, Su),
            e.owner ? (S(), et(fu, {
              key: 0,
              modelValue: o.value,
              "onUpdate:modelValue": f[1] || (f[1] = (p) => o.value = p),
              onToast: f[2] || (f[2] = (p) => s("toast", p))
            }, null, 8, ["modelValue"])) : de("", !0),
            r.value.length && !R.value ? (S(), I("span", $u, ne(r.value.length) + " 个文件 · " + ne(we(wt)(x.value)), 1)) : de("", !0)
          ]),
          g("div", Au, [
            g("span", {
              class: Ae(["connection-light", `connection-light--${e.connectionState}`]),
              role: "status",
              "aria-label": P.value,
              title: P.value
            }, null, 10, Mu),
            g("button", {
              class: Ae(["send-button", { "send-button--stop": E.value, "send-button--processing": c.value === "processing" }]),
              type: "button",
              disabled: c.value === "processing" || !R.value && !i.value.length && !r.value.length,
              "aria-label": c.value === "processing" ? "服务器正在保存" : E.value ? "取消上传" : "发送",
              title: c.value === "processing" ? "服务器正在保存" : E.value ? "取消上传" : "发送",
              onClick: f[3] || (f[3] = (p) => E.value ? zs() : it())
            }, [
              ee(ve, { name: H.value }, null, 8, ["name"])
            ], 10, ku)
          ])
        ]),
        C.value ? (S(), I("p", Lu, ne(C.value), 1)) : de("", !0)
      ], 34)
    ]));
  }
}), Fu = {
  class: "confirm-dialog",
  role: "alertdialog",
  "aria-modal": "true",
  "aria-labelledby": "confirm-title"
}, Ru = { class: "confirm-icon" }, Nu = { id: "confirm-title" }, Hu = { class: "confirm-actions" }, Bu = ["disabled"], Vu = ["disabled"], ju = /* @__PURE__ */ tt({
  __name: "ConfirmDialog",
  props: {
    open: { type: Boolean },
    title: {},
    copy: {},
    busy: { type: Boolean }
  },
  emits: ["cancel", "confirm"],
  setup(e, { emit: t }) {
    const n = t;
    return (s, i) => (S(), et(xn, { name: "dialog-fade" }, {
      default: Ut(() => [
        e.open ? (S(), I("div", {
          key: 0,
          class: "dialog-backdrop",
          role: "presentation",
          onPointerdown: i[2] || (i[2] = Le((r) => n("cancel"), ["self"]))
        }, [
          g("section", Fu, [
            g("div", Ru, [
              ee(ve, { name: "trash" })
            ]),
            g("h2", Nu, ne(e.title), 1),
            g("p", null, ne(e.copy), 1),
            g("div", Hu, [
              g("button", {
                class: "secondary-button",
                type: "button",
                disabled: e.busy,
                onClick: i[0] || (i[0] = (r) => n("cancel"))
              }, "取消", 8, Bu),
              g("button", {
                class: "danger-button",
                type: "button",
                disabled: e.busy,
                onClick: i[1] || (i[1] = (r) => n("confirm"))
              }, ne(e.busy ? "正在删除…" : "彻底删除"), 9, Vu)
            ])
          ])
        ], 32)) : de("", !0)
      ]),
      _: 1
    }));
  }
}), Bi = "drop.refresh-position.v1", Mn = 42, Ku = /* @__PURE__ */ tt({
  __name: "FloatingRefresh",
  props: {
    refreshing: { type: Boolean }
  },
  emits: ["refresh"],
  setup(e, { emit: t }) {
    const n = t, s = /* @__PURE__ */ z({ x: 20, y: 18 }), i = /* @__PURE__ */ z(!1);
    let r = "left", o = null, l = 0, a = 0, d = 0, c = 0, h = !1, v = !1, _ = null;
    const C = ge(() => ({ left: `${s.value.x}px`, top: `${s.value.y}px` }));
    function M() {
      return window.innerWidth <= 720 ? 11 : 20;
    }
    function F() {
      const x = M(), G = document.querySelector(".composer-box")?.getBoundingClientRect().top ?? window.innerHeight;
      return {
        minX: x,
        maxX: Math.max(x, window.innerWidth - Mn - x),
        minY: x,
        maxY: Math.max(x, Math.min(window.innerHeight - Mn - x, G - Mn - 12))
      };
    }
    function D(x, B, G) {
      return Math.min(G, Math.max(B, x));
    }
    function L() {
      try {
        const B = JSON.parse(localStorage.getItem(Bi) || "null");
        B?.side === "right" && (r = "right"), B?.side === "left" && (r = "left"), typeof B?.y == "number" && Number.isFinite(B.y) && (s.value.y = B.y);
      } catch {
      }
      const x = F();
      s.value = {
        x: r === "left" ? x.minX : x.maxX,
        y: D(s.value.y, x.minY, x.maxY)
      };
    }
    function N() {
      try {
        localStorage.setItem(Bi, JSON.stringify({ side: r, y: Math.round(s.value.y) }));
      } catch {
      }
    }
    function T(x) {
      x.button === 0 && (o = x.pointerId, l = x.clientX, a = x.clientY, d = s.value.x, c = s.value.y, h = !1, x.currentTarget.setPointerCapture(x.pointerId));
    }
    function K(x) {
      if (o !== x.pointerId) return;
      const B = x.clientX - l, G = x.clientY - a;
      if (!h && Math.hypot(B, G) < 5) return;
      h = !0, i.value = !0;
      const P = F();
      s.value = {
        x: D(d + B, P.minX, P.maxX),
        y: D(c + G, P.minY, P.maxY)
      };
    }
    function R(x) {
      if (o !== x.pointerId || (o = null, !h)) return;
      const B = F();
      r = s.value.x + Mn / 2 < window.innerWidth / 2 ? "left" : "right", s.value.x = r === "left" ? B.minX : B.maxX, s.value.y = D(s.value.y, B.minY, B.maxY), i.value = !1, v = !0, N(), window.setTimeout(() => {
        v = !1;
      }, 0);
    }
    function E(x) {
      if (v) {
        x.preventDefault();
        return;
      }
      n("refresh");
    }
    function H() {
      L();
    }
    return gt(() => {
      ht(() => {
        L();
        const x = document.querySelector(".composer-box");
        x && "ResizeObserver" in window && (_ = new ResizeObserver(L), _.observe(x));
      }), window.addEventListener("resize", H);
    }), xt(() => {
      _?.disconnect(), window.removeEventListener("resize", H);
    }), (x, B) => (S(), I("button", {
      class: Ae(["floating-refresh", { spinning: e.refreshing, dragging: i.value }]),
      style: Kt(C.value),
      type: "button",
      "aria-label": "刷新消息，可拖动位置",
      title: "刷新 · 可拖动",
      onPointerdown: T,
      onPointermove: K,
      onPointerup: R,
      onPointercancel: R,
      onClick: E
    }, [
      ee(ve, { name: "refresh" })
    ], 38));
  }
}), Uu = {
  key: 0,
  class: "attachment-visual"
}, Wu = ["aria-label"], qu = ["src", "alt"], Yu = {
  key: 0,
  class: "image-loading"
}, Xu = {
  key: 1,
  class: "attachment-visual attachment-video"
}, zu = ["src"], Gu = {
  key: 2,
  class: "file-glyph",
  "aria-hidden": "true"
}, Ju = { class: "attachment-details" }, Zu = ["title"], Qu = { class: "attachment-size" }, ec = ["href", "download", "aria-label"], tc = /* @__PURE__ */ tt({
  __name: "AttachmentCard",
  props: {
    attachment: {},
    role: {}
  },
  emits: ["toast", "openImage"],
  setup(e, { emit: t }) {
    const n = e, s = t, i = /* @__PURE__ */ z(!1), r = /* @__PURE__ */ z(!1), o = /* @__PURE__ */ z(!1), l = /* @__PURE__ */ z(!1), a = /* @__PURE__ */ z(!1), d = /* @__PURE__ */ z(!1), c = ge(() => n.attachment.mime_type.startsWith("video/"));
    function h(F) {
      i.value = !0, o.value = !1, l.value = F;
    }
    function v() {
      r.value = !0, l.value && nu(n.attachment.id);
    }
    function _() {
      i.value = !1, r.value = !1, o.value = !0, s("toast", "图片预览加载失败");
    }
    function C() {
      a.value = !0, d.value = !1;
    }
    function M() {
      a.value = !1, d.value = !0, s("toast", "视频加载失败，可尝试右侧下载按钮");
    }
    return gt(() => {
      const F = n.role === "owner" || n.role === "hermes";
      n.attachment.previewable && n.attachment.preview_url && (F || io().has(n.attachment.id)) && h(!1);
    }), (F, D) => (S(), I("div", {
      class: Ae(["attachment", { "attachment--image": e.attachment.previewable, "attachment--video": c.value }])
    }, [
      e.attachment.previewable ? (S(), I("div", Uu, [
        i.value ? (S(), I("button", {
          key: 0,
          class: Ae(["image-link", { "is-loaded": r.value }]),
          type: "button",
          "aria-label": `预览 ${e.attachment.original_name}`,
          onClick: D[0] || (D[0] = (L) => s("openImage", e.attachment))
        }, [
          g("img", {
            src: e.attachment.preview_url || e.attachment.download_url,
            alt: e.attachment.original_name,
            loading: "lazy",
            decoding: "async",
            onLoad: v,
            onError: _
          }, null, 40, qu),
          r.value ? de("", !0) : (S(), I("span", Yu, [...D[2] || (D[2] = [
            g("i", null, null, -1),
            Ys("正在加载预览", -1)
          ])]))
        ], 10, Wu)) : (S(), I("button", {
          key: 1,
          class: "preview-trigger",
          type: "button",
          onClick: D[1] || (D[1] = (L) => h(!0))
        }, [
          D[3] || (D[3] = g("span", { class: "preview-glyph" }, "IMG", -1)),
          g("span", null, ne(o.value ? "加载失败，点按重试" : e.attachment.preview_url ? "点按加载省流量预览" : "点按加载图片"), 1)
        ]))
      ])) : c.value ? (S(), I("div", Xu, [
        a.value ? (S(), I("video", {
          key: 0,
          src: e.attachment.download_url,
          controls: "",
          playsinline: "",
          preload: "metadata",
          onError: M
        }, null, 40, zu)) : (S(), I("button", {
          key: 1,
          class: "preview-trigger video-trigger",
          type: "button",
          onClick: C
        }, [
          D[4] || (D[4] = g("span", {
            class: "video-play",
            "aria-hidden": "true"
          }, "▶", -1)),
          g("span", null, ne(d.value ? "加载失败，点按重试" : "点按在页面内播放"), 1)
        ]))
      ])) : (S(), I("div", Gu, [
        g("span", null, ne(we(so)(e.attachment.original_name)), 1)
      ])),
      g("div", Ju, [
        g("span", {
          class: "attachment-name",
          title: e.attachment.original_name
        }, ne(e.attachment.original_name), 9, Zu),
        g("span", Qu, ne(we(wt)(e.attachment.size)), 1)
      ]),
      g("a", {
        class: "attachment-download",
        href: `${e.attachment.download_url}?download=1`,
        download: e.attachment.original_name,
        target: "_blank",
        rel: "noopener",
        "aria-label": `下载 ${e.attachment.original_name}`
      }, [
        ee(ve, { name: "download" })
      ], 8, ec)
    ], 2));
  }
}), nc = { class: "lightbox-header" }, sc = { class: "lightbox-title" }, ic = ["title"], rc = { key: 0 }, oc = ["href", "download", "aria-label"], lc = {
  key: 1,
  class: "lightbox-loading"
}, ac = {
  key: 2,
  class: "lightbox-failed"
}, uc = ["href"], cc = ["src", "alt"], fc = {
  key: 0,
  class: "lightbox-dots",
  "aria-hidden": "true"
}, dc = /* @__PURE__ */ tt({
  __name: "ImageLightbox",
  props: {
    images: {},
    initialIndex: {}
  },
  emits: ["close"],
  setup(e, { emit: t }) {
    const n = e, s = t, i = /* @__PURE__ */ z(n.initialIndex), r = /* @__PURE__ */ z(!0), o = /* @__PURE__ */ z(!1), l = /* @__PURE__ */ z(null);
    let a = null, d = "";
    const c = ge(() => n.images[i.value]), h = ge(() => n.images.length > 1);
    function v() {
      r.value = !0, o.value = !1;
    }
    function _(N) {
      n.images.length && (i.value = (N + n.images.length) % n.images.length, v());
    }
    function C() {
      _(i.value - 1);
    }
    function M() {
      _(i.value + 1);
    }
    function F(N) {
      N.key === "Escape" && s("close"), N.key === "ArrowLeft" && h.value && C(), N.key === "ArrowRight" && h.value && M();
    }
    function D(N) {
      h.value && (N.target.closest("button, a") || (a = N.clientX, l.value?.setPointerCapture(N.pointerId)));
    }
    function L(N) {
      if (a === null) return;
      const T = N.clientX - a;
      a = null, !(Math.abs(T) < 44) && (T > 0 ? C() : M());
    }
    return ln(() => n.initialIndex, (N) => {
      i.value = N, v();
    }), ln(i, () => {
      ht(() => l.value?.focus());
    }), gt(() => {
      d = document.body.style.overflow, document.body.style.overflow = "hidden", window.addEventListener("keydown", F), ht(() => l.value?.focus());
    }), xt(() => {
      document.body.style.overflow = d, window.removeEventListener("keydown", F);
    }), (N, T) => (S(), et(xr, { to: "body" }, [
      g("div", {
        class: "image-lightbox",
        role: "dialog",
        "aria-modal": "true",
        "aria-label": "图片预览",
        onClick: T[8] || (T[8] = Le((K) => s("close"), ["self"]))
      }, [
        g("header", nc, [
          g("div", sc, [
            g("strong", {
              title: c.value.original_name
            }, ne(c.value.original_name), 9, ic),
            h.value ? (S(), I("span", rc, ne(i.value + 1) + " / " + ne(e.images.length) + " · 左右滑动切换", 1)) : de("", !0)
          ]),
          g("a", {
            class: "lightbox-action",
            href: `${c.value.download_url}?download=1`,
            download: c.value.original_name,
            target: "_blank",
            rel: "noopener",
            "aria-label": `下载 ${c.value.original_name}`
          }, [
            ee(ve, { name: "download" })
          ], 8, oc),
          g("button", {
            class: "lightbox-action",
            type: "button",
            "aria-label": "关闭图片预览",
            onClick: T[0] || (T[0] = (K) => s("close"))
          }, [
            ee(ve, { name: "close" })
          ])
        ]),
        g("div", {
          ref_key: "stage",
          ref: l,
          class: "lightbox-stage",
          tabindex: "-1",
          onPointerdown: D,
          onPointerup: L,
          onPointercancel: T[7] || (T[7] = (K) => /* @__PURE__ */ _e(a) ? a.value = null : a = null)
        }, [
          h.value ? (S(), I("button", {
            key: 0,
            class: "lightbox-nav lightbox-nav--previous",
            type: "button",
            "aria-label": "上一张",
            onPointerdown: T[1] || (T[1] = Le(() => {
            }, ["stop"])),
            onPointerup: T[2] || (T[2] = Le(() => {
            }, ["stop"])),
            onClick: Le(C, ["stop"])
          }, "‹", 32)) : de("", !0),
          r.value ? (S(), I("div", lc, [...T[9] || (T[9] = [
            g("i", null, null, -1),
            g("span", null, "正在加载预览", -1)
          ])])) : de("", !0),
          o.value ? (S(), I("div", ac, [
            T[10] || (T[10] = g("strong", null, "这张图片暂时无法预览", -1)),
            g("a", {
              href: `${c.value.download_url}?download=1`,
              target: "_blank",
              rel: "noopener"
            }, "下载原图", 8, uc)
          ])) : de("", !0),
          vr((S(), I("img", {
            key: c.value.id,
            src: c.value.preview_url || c.value.download_url,
            alt: c.value.original_name,
            draggable: "false",
            onLoad: T[3] || (T[3] = (K) => r.value = !1),
            onError: T[4] || (T[4] = (K) => {
              r.value = !1, o.value = !0;
            })
          }, null, 40, cc)), [
            [da, !o.value]
          ]),
          h.value ? (S(), I("button", {
            key: 3,
            class: "lightbox-nav lightbox-nav--next",
            type: "button",
            "aria-label": "下一张",
            onPointerdown: T[5] || (T[5] = Le(() => {
            }, ["stop"])),
            onPointerup: T[6] || (T[6] = Le(() => {
            }, ["stop"])),
            onClick: Le(M, ["stop"])
          }, "›", 32)) : de("", !0)
        ], 544),
        h.value ? (S(), I("div", fc, [
          (S(!0), I(te, null, ft(e.images.length, (K) => (S(), I("i", {
            key: K,
            class: Ae({ active: K - 1 === i.value })
          }, null, 2))), 128))
        ])) : de("", !0)
      ])
    ]));
  }
}), hc = ["data-item-id"], pc = { class: "drop-card-content" }, gc = ["href"], mc = { class: "drop-meta" }, vc = ["datetime"], yc = { key: 0 }, bc = {
  key: 0,
  class: "card-actions"
}, wc = ["aria-expanded"], _c = ["onClick"], xc = /* @__PURE__ */ tt({
  __name: "MessageCard",
  props: {
    item: {},
    role: {}
  },
  emits: ["copy", "expiry", "remove", "toast"],
  setup(e, { emit: t }) {
    const n = e, s = t, i = /* @__PURE__ */ z(!1), r = /* @__PURE__ */ z("main"), o = /* @__PURE__ */ z("down"), l = /* @__PURE__ */ z(null), a = /* @__PURE__ */ z(null), d = ge(() => n.role === "owner" || n.role === "hermes"), c = ge(() => d.value), h = ge(() => tu(n.item.text_preview || "")), v = ge(() => n.item.attachments.filter((R) => R.previewable));
    function _(R) {
      i.value && a.value && !a.value.contains(R.target) && C();
    }
    function C() {
      i.value = !1, r.value = "main";
    }
    function M() {
      if (!a.value || window.innerWidth > 720) {
        o.value = "down";
        return;
      }
      const R = a.value.getBoundingClientRect(), E = document.querySelector(".composer-box")?.getBoundingClientRect().top ?? window.innerHeight, H = r.value === "expiry" ? 206 : 166;
      o.value = R.bottom + H > E - 10 ? "up" : "down";
    }
    async function F() {
      if (i.value) {
        C();
        return;
      }
      r.value = "main", i.value = !0, await ht(), M();
    }
    async function D() {
      r.value = "expiry", await ht(), M();
    }
    function L(R) {
      const E = v.value.findIndex((H) => H.id === R.id);
      l.value = E < 0 ? 0 : E;
    }
    function N(R) {
      C(), s("expiry", n.item.id, R);
    }
    function T(R) {
      if (R?.target?.closest("a")) return;
      const H = window.getSelection();
      H && !H.isCollapsed && H.toString().trim() || s("copy", n.item);
    }
    function K(R) {
      R.key !== "Enter" && R.key !== " " || (R.preventDefault(), T());
    }
    return gt(() => document.addEventListener("pointerdown", _)), xt(() => document.removeEventListener("pointerdown", _)), (R, E) => (S(), I("article", {
      ref_key: "card",
      ref: a,
      class: "drop-card",
      "data-item-id": e.item.id
    }, [
      g("div", pc, [
        e.item.has_text ? (S(), I("p", {
          key: 0,
          class: Ae(["drop-text drop-text--copyable", { "drop-text--truncated": e.item.text_truncated }]),
          role: "button",
          tabindex: "0",
          "aria-label": "复制全文",
          title: "点击复制全文",
          onClick: T,
          onKeydown: K
        }, [
          (S(!0), I(te, null, ft(h.value, (H, x) => (S(), I(te, { key: x }, [
            H.type === "link" ? (S(), I("a", {
              key: 0,
              href: H.value,
              target: "_blank",
              rel: "noopener noreferrer",
              onClick: E[0] || (E[0] = Le(() => {
              }, ["stop"]))
            }, ne(H.value), 9, gc)) : (S(), I(te, { key: 1 }, [
              Ys(ne(H.value), 1)
            ], 64))
          ], 64))), 128))
        ], 34)) : de("", !0),
        e.item.attachments?.length ? (S(), I("div", {
          key: 1,
          class: Ae(["attachment-grid", { "attachment-grid--single": e.item.attachments.length === 1 }])
        }, [
          (S(!0), I(te, null, ft(e.item.attachments, (H) => (S(), et(tc, {
            key: H.id,
            attachment: H,
            role: e.role,
            onOpenImage: L,
            onToast: E[1] || (E[1] = (x) => s("toast", x))
          }, null, 8, ["attachment", "role"]))), 128))
        ], 2)) : de("", !0),
        g("div", mc, [
          g("time", {
            datetime: e.item.created_at
          }, ne(we(Za)(e.item.created_at)), 9, vc),
          g("span", null, ne(we(Ni)(e.item.expires_at)), 1),
          e.item.total_size && (e.item.has_text || e.item.attachments.length > 1) ? (S(), I("span", yc, ne(we(wt)(e.item.total_size)), 1)) : de("", !0)
        ])
      ]),
      c.value ? (S(), I("div", bc, [
        g("button", {
          class: "quiet-icon-button",
          type: "button",
          "aria-label": "更多操作",
          "aria-haspopup": "menu",
          "aria-expanded": i.value,
          onPointerdown: E[2] || (E[2] = Le(() => {
          }, ["stop"])),
          onClick: F
        }, [
          ee(ve, { name: "more" })
        ], 40, wc),
        ee(xn, { name: "menu-pop" }, {
          default: Ut(() => [
            i.value ? (S(), I("div", {
              key: 0,
              class: Ae(["card-menu", `card-menu--${o.value}`]),
              role: "menu",
              onPointerdown: E[6] || (E[6] = Le(() => {
              }, ["stop"]))
            }, [
              r.value === "main" ? (S(), I(te, { key: 0 }, [
                e.item.has_text ? (S(), I("button", {
                  key: 0,
                  type: "button",
                  role: "menuitem",
                  onClick: E[3] || (E[3] = (H) => {
                    C(), s("copy", e.item);
                  })
                }, [
                  ee(ve, { name: "copy" }),
                  E[8] || (E[8] = g("span", null, "复制全文", -1))
                ])) : de("", !0),
                g("button", {
                  class: "menu-expiry-action",
                  type: "button",
                  role: "menuitem",
                  onClick: D
                }, [
                  ee(ve, { name: "clock" }),
                  g("span", null, [
                    E[9] || (E[9] = g("strong", null, "有效期", -1)),
                    g("small", null, ne(we(Ni)(e.item.expires_at)), 1)
                  ]),
                  E[10] || (E[10] = g("span", {
                    class: "menu-chevron",
                    "aria-hidden": "true"
                  }, "›", -1))
                ]),
                E[12] || (E[12] = g("div", { class: "menu-separator" }, null, -1)),
                g("button", {
                  class: "danger-action",
                  type: "button",
                  role: "menuitem",
                  onClick: E[4] || (E[4] = (H) => {
                    C(), s("remove", e.item);
                  })
                }, [
                  ee(ve, { name: "trash" }),
                  E[11] || (E[11] = g("span", null, "彻底删除", -1))
                ])
              ], 64)) : (S(), I(te, { key: 1 }, [
                g("button", {
                  class: "menu-back",
                  type: "button",
                  role: "menuitem",
                  onClick: E[5] || (E[5] = (H) => r.value = "main")
                }, [
                  ee(ve, { name: "back" }),
                  E[13] || (E[13] = g("span", null, "调整有效期", -1))
                ]),
                E[14] || (E[14] = g("div", { class: "menu-separator" }, null, -1)),
                (S(!0), I(te, null, ft(we(no), (H) => (S(), I("button", {
                  key: H,
                  type: "button",
                  role: "menuitem",
                  onClick: (x) => N(H)
                }, [
                  ee(ve, { name: "clock" }),
                  g("span", null, ne(we(Un)(H)), 1)
                ], 8, _c))), 128))
              ], 64))
            ], 34)) : de("", !0)
          ]),
          _: 1
        })
      ])) : de("", !0),
      l.value !== null ? (S(), et(dc, {
        key: 1,
        images: v.value,
        "initial-index": l.value,
        onClose: E[7] || (E[7] = (H) => l.value = null)
      }, null, 8, ["images", "initial-index"])) : de("", !0)
    ], 8, hc));
  }
}), Tc = { class: "workspace" }, Cc = {
  class: "timeline",
  "aria-label": "临时消息"
}, Ec = {
  key: 1,
  class: "timeline-feed"
}, Sc = { class: "day-divider" }, $c = { class: "day-items" }, Ac = {
  key: 2,
  class: "empty-state"
}, Mc = {
  key: 0,
  class: "toast",
  role: "status",
  "aria-live": "polite"
}, kc = /* @__PURE__ */ tt({
  __name: "DropWorkspace",
  props: {
    role: {}
  },
  setup(e) {
    const t = e, n = /* @__PURE__ */ z([]), s = /* @__PURE__ */ z(!0), i = /* @__PURE__ */ z(!1), r = /* @__PURE__ */ z(!1), o = /* @__PURE__ */ z(!1), l = /* @__PURE__ */ z(typeof navigator > "u" ? !0 : navigator.onLine), a = /* @__PURE__ */ z(""), d = /* @__PURE__ */ z(null), c = /* @__PURE__ */ z(!1);
    let h = 0, v = 0, _ = null, C = null;
    const M = ge(() => t.role === "owner" || t.role === "hermes"), F = ge(() => l.value ? r.value ? "connected" : o.value ? "disconnected" : "connecting" : "offline"), D = ge(() => {
      const W = [], J = [...n.value].sort((Z, Y) => new Date(Z.created_at).getTime() - new Date(Y.created_at).getTime() || Z.id.localeCompare(Y.id));
      for (const Z of J) {
        const Y = tn(Z.created_at);
        let Q = W[W.length - 1];
        (!Q || Q.key !== Y) && (Q = { key: Y, label: Qa(Z.created_at), items: [] }, W.push(Q)), Q.items.push(Z);
      }
      return W;
    });
    function L(W) {
      a.value = W, window.clearTimeout(v), v = window.setTimeout(() => {
        a.value = "";
      }, 2400);
    }
    function N() {
      return document.documentElement.scrollHeight - window.innerHeight - window.scrollY < 180;
    }
    function T(W = "auto") {
      window.scrollTo({ top: document.documentElement.scrollHeight, behavior: W });
    }
    async function K(W = !1, J = !1) {
      const Z = J || s.value || N();
      C?.abort();
      const Y = new AbortController();
      C = Y, W && (i.value = !0);
      try {
        n.value = await Ba(Y.signal), s.value = !1, Z && await ht(() => T()), W && L("已刷新");
      } catch (Q) {
        if (Y.signal.aborted) return;
        L(nn(Q));
      } finally {
        C === Y && (C = null, s.value = !1, i.value = !1);
      }
    }
    async function R() {
      await K(!1, !0);
    }
    function E() {
      window.clearTimeout(h), h = window.setTimeout(() => {
        K();
      }, 140);
    }
    async function H(W) {
      try {
        if (!W.full_text_url) throw new Error("没有可复制的文字");
        const J = await Va(W.full_text_url);
        if (!J.ok) throw new Error(`读取文字失败 (${J.status})`);
        await navigator.clipboard.writeText(await J.text()), L("已复制全文");
      } catch (J) {
        L(nn(J));
      }
    }
    async function x(W, J) {
      try {
        await ja(W, J), L(`有效期已改为 ${Un(J)}`), E();
      } catch (Z) {
        L(nn(Z));
      }
    }
    async function B() {
      if (!(!d.value || c.value)) {
        c.value = !0;
        try {
          const W = d.value.id;
          await Ka(W), n.value = n.value.filter((J) => J.id !== W), d.value = null, L("已彻底删除");
        } catch (W) {
          L(nn(W));
        } finally {
          c.value = !1;
        }
      }
    }
    function G() {
      _?.close(), r.value = !1, o.value = !1, _ = new EventSource(Wt("/api/v1/events")), _.addEventListener("open", () => {
        r.value = !0, o.value = !1;
      }), _.addEventListener("sync", E), _.addEventListener("items_changed", E), _.onerror = () => {
        r.value = !1, o.value = !0;
      };
    }
    function P() {
      l.value = !1, r.value = !1;
    }
    function ie() {
      l.value = !0, G();
    }
    return gt(() => {
      K(), G(), window.addEventListener("offline", P), window.addEventListener("online", ie);
    }), xt(() => {
      _?.close(), C?.abort(), window.clearTimeout(h), window.clearTimeout(v), window.removeEventListener("offline", P), window.removeEventListener("online", ie);
    }), (W, J) => (S(), I("main", Tc, [
      ee(Ku, {
        refreshing: i.value,
        onRefresh: J[0] || (J[0] = (Z) => K(!0))
      }, null, 8, ["refreshing"]),
      g("section", Cc, [
        s.value ? (S(), I(te, { key: 0 }, [
          J[4] || (J[4] = g("div", { class: "day-divider" }, [
            g("span", null, "正在读取")
          ], -1)),
          (S(), I(te, null, ft(3, (Z) => g("div", {
            key: Z,
            class: "skeleton-card"
          }, [...J[3] || (J[3] = [
            g("i", null, null, -1),
            g("span", null, null, -1),
            g("small", null, null, -1)
          ])])), 64))
        ], 64)) : D.value.length ? (S(), I("div", Ec, [
          (S(!0), I(te, null, ft(D.value, (Z) => (S(), I("section", {
            key: Z.key,
            class: "day-group"
          }, [
            g("div", Sc, [
              g("span", null, ne(Z.label), 1)
            ]),
            g("div", $c, [
              (S(!0), I(te, null, ft(Z.items, (Y) => (S(), et(xc, {
                key: Y.id,
                item: Y,
                role: e.role,
                onCopy: H,
                onExpiry: x,
                onRemove: J[1] || (J[1] = (Q) => d.value = Q),
                onToast: L
              }, null, 8, ["item", "role"]))), 128))
            ])
          ]))), 128))
        ])) : (S(), I("section", Ac, [...J[5] || (J[5] = [
          g("div", { class: "empty-mark" }, [
            g("span")
          ], -1),
          g("h1", null, "这里还很安静", -1),
          g("p", null, "粘贴一段文字、截图，或者添加文件。", -1)
        ])]))
      ]),
      ee(Du, {
        owner: M.value,
        "connection-state": F.value,
        onSent: R,
        onToast: L
      }, null, 8, ["owner", "connection-state"]),
      ee(ju, {
        open: !!d.value,
        title: "删除这条消息？",
        copy: "消息和全部附件会立即永久删除，此操作无法恢复。",
        busy: c.value,
        onCancel: J[2] || (J[2] = (Z) => d.value = null),
        onConfirm: B
      }, null, 8, ["open", "busy"]),
      ee(xn, { name: "toast-rise" }, {
        default: Ut(() => [
          a.value ? (S(), I("div", Mc, ne(a.value), 1)) : de("", !0)
        ]),
        _: 1
      })
    ]));
  }
}), Lc = /* @__PURE__ */ tt({
  __name: "App",
  props: {
    page: {},
    role: {}
  },
  setup(e) {
    return (t, n) => (S(), et(kc, { role: e.role }, null, 8, ["role"]));
  }
}), In = document.querySelector("#app");
if (!In) throw new Error("Drop root element is missing");
Da(Lc, {
  page: In.dataset.page || "app",
  role: In.dataset.role || "guest"
}).mount(In);
