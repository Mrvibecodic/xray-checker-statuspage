<!DOCTYPE html>
<html lang="ru" data-theme="__THEME__">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<meta name="description" content="__DESC__">
<title>__TITLE__</title>
__FAVICON__
<style>
@font-face{font-family:'Inter';font-style:normal;font-weight:400;font-display:swap;src:url('fonts/inter-cyrillic-400-normal.woff2') format('woff2');unicode-range:U+0301,U+0400-045F,U+0490-0491,U+04B0-04B1,U+2116;}
@font-face{font-family:'Inter';font-style:normal;font-weight:500;font-display:swap;src:url('fonts/inter-cyrillic-500-normal.woff2') format('woff2');unicode-range:U+0301,U+0400-045F,U+0490-0491,U+04B0-04B1,U+2116;}
@font-face{font-family:'Inter';font-style:normal;font-weight:600;font-display:swap;src:url('fonts/inter-cyrillic-600-normal.woff2') format('woff2');unicode-range:U+0301,U+0400-045F,U+0490-0491,U+04B0-04B1,U+2116;}
@font-face{font-family:'Inter';font-style:normal;font-weight:400;font-display:swap;src:url('fonts/inter-latin-400-normal.woff2') format('woff2');unicode-range:U+0000-00FF,U+0131,U+0152-0153,U+02BB-02BC,U+02C6,U+02DA,U+02DC,U+2000-206F,U+2074,U+20AC,U+2122,U+2191,U+2193,U+2212,U+2215,U+FEFF,U+FFFD;}
@font-face{font-family:'Inter';font-style:normal;font-weight:500;font-display:swap;src:url('fonts/inter-latin-500-normal.woff2') format('woff2');unicode-range:U+0000-00FF,U+0131,U+0152-0153,U+02BB-02BC,U+02C6,U+02DA,U+02DC,U+2000-206F,U+2074,U+20AC,U+2122,U+2191,U+2193,U+2212,U+2215,U+FEFF,U+FFFD;}
@font-face{font-family:'Inter';font-style:normal;font-weight:600;font-display:swap;src:url('fonts/inter-latin-600-normal.woff2') format('woff2');unicode-range:U+0000-00FF,U+0131,U+0152-0153,U+02BB-02BC,U+02C6,U+02DA,U+02DC,U+2000-206F,U+2074,U+20AC,U+2122,U+2191,U+2193,U+2212,U+2215,U+FEFF,U+FFFD;}
:root,html[data-theme="light"]{
  --bg:#f6f8fb; --card:#ffffff; --soft:#f2f5f9; --line:#dce3ec; --hover:#f7f9fc;
  --tx:#111827; --tx2:#4b5563; --tx3:#737d8c;
  --ok:#12a370; --warn:#d58a10; --orange:#d65f2b; --bad:#dc3f3d; --info:#2563eb;
  --okbg:rgba(18,163,112,.12); --warnbg:rgba(213,138,16,.13); --badbg:rgba(220,63,61,.12); --infobg:rgba(37,99,235,.11);
  --shadow:0 18px 55px rgba(15,23,42,.08),0 1px 2px rgba(15,23,42,.06);
}
@media (prefers-color-scheme: dark){
  :root{--bg:#0f141b; --card:#171d26; --soft:#1f2732; --line:#2d3745; --hover:#202938;
    --tx:#f2f6fb; --tx2:#b8c2cf; --tx3:#8792a2;
    --ok:#20c087; --warn:#efb443; --orange:#f07a44; --bad:#f15c5a; --info:#69a1ff;
    --okbg:rgba(32,192,135,.14); --warnbg:rgba(239,180,67,.14); --badbg:rgba(241,92,90,.14); --infobg:rgba(105,161,255,.13);
    --shadow:0 22px 70px rgba(0,0,0,.32);}
}
html[data-theme="dark"]{--bg:#0f141b; --card:#171d26; --soft:#1f2732; --line:#2d3745; --hover:#202938;
  --tx:#f2f6fb; --tx2:#b8c2cf; --tx3:#8792a2;
  --ok:#20c087; --warn:#efb443; --orange:#f07a44; --bad:#f15c5a; --info:#69a1ff;
  --okbg:rgba(32,192,135,.14); --warnbg:rgba(239,180,67,.14); --badbg:rgba(241,92,90,.14); --infobg:rgba(105,161,255,.13);
  --shadow:0 22px 70px rgba(0,0,0,.32);}
*{box-sizing:border-box}
html{overflow-y:scroll;scrollbar-gutter:stable;}
body{margin:0;background:var(--bg);color:var(--tx);
  font-family:'Inter',-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,sans-serif;
  -webkit-font-smoothing:antialiased;text-rendering:optimizeLegibility;}
body::before{content:"";position:fixed;inset:0;z-index:-1;pointer-events:none;background:
  radial-gradient(900px 420px at 12% -10%,color-mix(in srgb,var(--info) 13%,transparent),transparent 64%),
  radial-gradient(820px 420px at 88% -8%,color-mix(in srgb,var(--ok) 10%,transparent),transparent 62%);}
@keyframes fadeUp{from{opacity:0;transform:translateY(7px);}to{opacity:1;transform:none;}}
@keyframes pulse{0%{box-shadow:0 0 0 0 rgba(22,176,122,.45);}70%{box-shadow:0 0 0 6px rgba(22,176,122,0);}100%{box-shadow:0 0 0 0 rgba(22,176,122,0);}}
@media (prefers-reduced-motion: reduce){*{animation:none !important;transition:none !important;}}
.wrap{max-width:1180px;margin:0 auto;padding:34px 20px 56px;}
.top{display:flex;align-items:center;justify-content:space-between;gap:16px;flex-wrap:wrap;margin-bottom:18px;
  background:linear-gradient(180deg,color-mix(in srgb,var(--card) 96%,transparent),color-mix(in srgb,var(--soft) 60%,var(--card)));
  border:1px solid var(--line);border-radius:22px;padding:22px 24px;box-shadow:var(--shadow);}
.brand{display:flex;align-items:center;gap:14px;min-width:0;}
.logo{width:46px;height:46px;border-radius:14px;background:var(--infobg);color:var(--info);
  display:flex;align-items:center;justify-content:center;overflow:hidden;}
.logo img{width:100%;height:100%;object-fit:contain;display:block;}
.brand h1{font-size:24px;font-weight:700;margin:0;line-height:1.15;letter-spacing:0;}
.brand p{font-size:13px;color:var(--tx2);margin:5px 0 0;line-height:1.35;}
.pill{display:flex;align-items:center;gap:8px;padding:8px 15px;border-radius:999px;
  font-size:13.5px;font-weight:650;border:1px solid transparent;white-space:nowrap;}
.pill .dot{width:8px;height:8px;border-radius:50%;display:inline-block;}
.pill.ok{background:var(--okbg);color:var(--ok);border-color:color-mix(in srgb,var(--ok) 24%,transparent);}
.pill.bad{background:var(--badbg);color:var(--bad);border-color:color-mix(in srgb,var(--bad) 24%,transparent);}
.pill.ok .dot{animation:pulse 2.4s ease-out infinite;}
.topr{display:flex;align-items:center;gap:10px;}
.stats{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px;margin-bottom:16px;}
.stat{background:var(--card);border:1px solid var(--line);border-radius:16px;padding:16px 18px;box-shadow:var(--shadow);min-width:0;}
.stat .l{font-size:12px;color:var(--tx2);font-weight:650;text-transform:uppercase;letter-spacing:.03em;}
.stat .v{font-size:27px;font-weight:750;margin-top:7px;letter-spacing:0;line-height:1.1;}
.stat .v .vsub{font-size:14px;font-weight:600;color:var(--info);margin-left:7px;letter-spacing:0;white-space:nowrap;vertical-align:1px;}
.servers{margin-top:2px;}
.section-head{display:flex;align-items:flex-end;justify-content:space-between;gap:12px;margin:18px 2px 10px;}
.section-head h2{margin:0;font-size:15px;letter-spacing:.02em;text-transform:uppercase;color:var(--tx2);}
.section-head p{margin:4px 0 0;font-size:13px;color:var(--tx3);}
.item{background:var(--card);border:1px solid var(--line);border-radius:15px;margin-bottom:9px;
  box-shadow:var(--shadow);overflow:hidden;animation:fadeUp .34s ease both;
  transition:border-color .15s, box-shadow .15s;}
.item:hover{border-color:color-mix(in srgb,var(--info) 32%,var(--line));box-shadow:0 16px 44px rgba(15,23,42,.10);}
.row{display:grid;grid-template-columns:260px minmax(260px,1fr) 138px 30px;align-items:center;gap:16px;padding:13px 16px;cursor:pointer;transition:background .14s;}
.row:hover{background:var(--hover);}
.label{display:flex;align-items:center;gap:10px;min-width:0;}
.flag{width:23px;height:16px;border-radius:3px;object-fit:cover;border:1px solid rgba(0,0,0,.12);flex:none;background:var(--soft);}
.inc-aff img{width:16px;height:11px;border-radius:2px;object-fit:cover;vertical-align:-1px;margin-right:3px;border:1px solid rgba(0,0,0,.12);flex:none;}
.nm{min-width:0;}
.name{display:flex;align-items:center;gap:8px;font-size:15px;font-weight:650;}
.name .sdot{width:9px;height:9px;border-radius:50%;flex:none;transition:background-color .3s;}
.name span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;}
.bars{display:block;height:28px;min-width:0;width:100%;}
.bars rect{transition:fill .3s, opacity .12s;cursor:pointer;}
.bars rect:hover{opacity:.55;}
.stat2{text-align:right;}
.stat2 .p{font-size:15px;font-weight:750;}
.stat2 .s{font-size:12px;color:var(--tx3);}
.chev{color:var(--tx3);margin-left:2px;flex:none;display:flex;align-items:center;justify-content:center;
  width:26px;height:26px;border-radius:50%;transition:transform .22s ease, background-color .15s, color .15s;}
.row:hover .chev{background:rgba(127,140,158,.14);color:var(--tx2);}
.item.open .chev{transform:rotate(180deg);}
.panel{max-height:0;overflow:hidden;opacity:0;background:var(--soft);border-top:1px solid transparent;
  padding:0 20px;transition:max-height .3s ease, opacity .24s ease, padding .28s ease, border-color .28s ease;}
.item.open .panel{opacity:1;padding:20px 20px 22px;border-top-color:var(--line);}
.phead{font-size:13px;color:var(--tx2);margin-bottom:16px;line-height:1.5;}
.tstats{display:grid;grid-template-columns:repeat(4,minmax(120px,1fr));gap:12px;margin-bottom:4px;}
.tstats div{display:flex;flex-direction:column;gap:4px;background:color-mix(in srgb,var(--card) 54%,transparent);border:1px solid color-mix(in srgb,var(--line) 70%,transparent);border-radius:12px;padding:10px 12px;}
.tstats span{font-size:12.5px;color:var(--tx2);}
.tstats b{font-size:18px;font-weight:600;letter-spacing:-.2px;}
.tcaption{font-size:12.5px;color:var(--tx3);margin:20px 0 9px;}
.tchartwrap{width:100%;position:relative;}
.tscroll{overflow-x:auto;overflow-y:hidden;scrollbar-width:thin;scrollbar-color:var(--line) transparent;}
.tscroll::-webkit-scrollbar{height:9px;}
.tscroll::-webkit-scrollbar-track{background:transparent;}
.tscroll::-webkit-scrollbar-thumb{background:var(--line);border-radius:9px;border:2px solid transparent;background-clip:content-box;}
.tscroll:hover::-webkit-scrollbar-thumb{background:var(--tx3);background-clip:content-box;}
.tcanvas{position:relative;padding-bottom:10px;}
.tchart{display:block;width:100%;height:150px;}
.tyaxis{position:absolute;right:6px;z-index:2;transform:translateY(-50%);font-size:12px;color:var(--tx3);
  background:var(--soft);padding:0 4px;pointer-events:none;border-radius:3px;}
.taxis{display:flex;justify-content:space-between;font-size:12px;color:var(--tx3);margin-top:9px;}
.empty{color:var(--tx2);font-size:13px;padding:6px 0;}
.legend{display:flex;align-items:center;gap:16px;flex-wrap:wrap;margin-top:14px;font-size:13px;color:var(--tx2);
  background:color-mix(in srgb,var(--card) 72%,transparent);border:1px solid var(--line);border-radius:14px;padding:11px 13px;}
.legend i{width:12px;height:12px;border-radius:3px;display:inline-block;margin-right:6px;vertical-align:-1px;}
.legend .right{margin-left:auto;}
.foot{margin-top:22px;font-size:13px;color:var(--tx3);text-align:center;}
#tip{position:fixed;pointer-events:none;opacity:0;transition:opacity .08s;z-index:60;
  background:var(--card);border:1px solid var(--line);border-radius:10px;padding:9px 11px;
  font-size:13px;min-width:140px;box-shadow:0 8px 28px rgba(18,28,45,.16);}
#tip .d{font-weight:600;margin-bottom:4px;}
#tip .k{color:var(--tx2);line-height:1.5;}
.skel{color:var(--tx2);font-size:14px;padding:30px 0;text-align:center;background:var(--card);border:1px solid var(--line);border-radius:14px;}
@media (max-width:900px){
  .stats{grid-template-columns:repeat(2,1fr);}
  .row{grid-template-columns:minmax(0,1fr) 132px 30px;gap:10px 12px;}
  .bars{grid-column:1 / -1;grid-row:2;height:27px;}
  .label{grid-column:1;}
  .stat2{grid-column:2;}
  .chev{grid-column:3;}
  .tstats{grid-template-columns:repeat(2,minmax(120px,1fr));}
}
@media (max-width:560px){
  .wrap{padding:22px 14px 40px;}
  .top{padding:16px;border-radius:18px;}
  .brand h1{font-size:18px;} .brand p{font-size:12px;}
  .stats{grid-template-columns:1fr 1fr;gap:10px;}
  .stat{padding:12px 14px;border-radius:12px;} .stat .v{font-size:22px;}
  .row{padding:12px 14px;}
  .stat2{width:auto;text-align:right;}
  .legend{gap:9px 14px;margin-top:14px;} .legend .right{display:none;}
  .tstats{grid-template-columns:repeat(2,minmax(0,1fr));gap:10px;}
  .tstats div{padding:9px 10px;}
  .tstats span{font-size:11.5px;line-height:1.25;}
  .tstats b{font-size:17px;}
  .panel{padding:0 14px;}
  .item.open .panel{padding:16px 14px 18px;}
}
#incidents{display:flex;flex-direction:column;gap:10px;margin:2px 0 14px}
.inc-card{border-radius:15px;padding:12px 16px;border:1px solid color-mix(in srgb,var(--warn) 30%,var(--line));border-left:5px solid var(--warn);background:color-mix(in srgb,var(--warn) 10%,var(--card));box-shadow:var(--shadow)}
.inc-card.inc-major{border-color:color-mix(in srgb,var(--orange) 32%,var(--line));border-left-color:var(--orange);background:color-mix(in srgb,var(--orange) 10%,var(--card))}
.inc-card.inc-critical{border-color:color-mix(in srgb,var(--bad) 34%,var(--line));border-left-color:var(--bad);background:color-mix(in srgb,var(--bad) 11%,var(--card))}
.inc-h{display:flex;gap:10px;align-items:center;font-size:12px;color:var(--tx2);margin-bottom:4px}
.inc-sev{font-weight:750}.inc-status{font-weight:650;color:var(--tx)}
.inc-title{font-weight:750;font-size:15px;}
.inc-time{margin-left:auto;font-size:11.5px;opacity:.8;font-weight:500;white-space:nowrap}
.inc-aff{margin-top:4px;font-size:12px;opacity:.85}
.inc-tl{margin-top:6px;display:flex;flex-direction:column;gap:3px;border-top:1px solid color-mix(in srgb,currentColor 12%,transparent);padding-top:6px}
.inc-tlrow{font-size:12px;display:flex;gap:7px;align-items:baseline;flex-wrap:wrap}
.inc-tlt{font-variant-numeric:tabular-nums;opacity:.7;min-width:38px;font-weight:500}
.inc-tls{font-weight:600}
.mnt-badge{margin-left:8px;font-size:11px;padding:1px 7px;border-radius:8px;border:1px solid var(--info);color:var(--info);white-space:nowrap}
.mnt-line{display:flex;align-items:center;gap:5px;margin-top:3px;font-size:11.5px;color:var(--info);font-weight:500;white-space:nowrap}
.item.maint{border-left:3px solid var(--info);background:color-mix(in srgb,var(--info) 7%,var(--card))}
.item.maint .bars{opacity:.35}
.item.maint .p{color:var(--info) !important}
</style>
</head>
<body>
<div class="wrap">
  <header class="top">
    <div class="brand">
      <div class="logo">__LOGO__</div>
      <div><h1 id="title">__TITLE__</h1><p id="subtitle">__SUBTITLE__</p></div>
    </div>
    <div class="topr">
      <div id="overall" class="pill ok"><span class="dot"></span><span>Загрузка...</span></div>
    </div>
  </header>
  <main>
    <section class="stats" aria-label="Сводка">
      <div class="stat"><div class="l">Активные серверы</div><div class="v" id="s-online">—</div></div>
      <div class="stat"><div class="l">Аптайм за __DAYS__ дн</div><div class="v" id="s-uptime">—</div></div>
      <div class="stat"><div class="l">Средний пинг</div><div class="v" id="s-ping">—</div></div>
      <div class="stat"><div class="l">Последняя проверка</div><div class="v" id="s-fresh">—</div></div>
    </section>
    <section id="incidents" aria-live="polite"></section>
    <section class="servers" aria-label="Серверы">
      <div class="section-head">
        <div>
          <h2>Серверы</h2>
          <p>30-дневная история доступности, текущая задержка и детализация по клику.</p>
        </div>
      </div>
      <div id="list"><div class="skel">Загрузка данных...</div></div>
      <div class="legend">
        <span><i style="background:#16b07a"></i>норма</span>
        <span><i style="background:#f0a82a"></i>до 30 мин</span>
        <span><i style="background:#e3692f"></i>30 мин – 2 ч</span>
        <span><i style="background:#e8504e"></i>от 2 ч</span>
        <span><i style="background:#cfd6df"></i>нет данных</span>
        <span class="right">← __DAYS__ дней назад · сегодня →</span>
      </div>
    </section>
    <div class="foot" id="foot"></div>
  </main>
</div>
<div id="tip"></div>
<script>
var GREY="#cfd6df";
var SVGNS="http://www.w3.org/2000/svg";
function colorFor(d){
  if(!d.hasData) return GREY;
  if(d.downMin<=0) return "#16b07a";
  if(d.downMin<=30) return "#f0a82a";
  if(d.downMin<120) return "#e3692f";
  return "#e8504e";
}
function fmtDur(m){
  if(m<=0) return "0 мин";
  if(m>=1440){var dd=Math.floor(m/1440),h=Math.round((m%1440)/60);return dd+" дн"+(h?" "+h+" ч":"");}
  if(m>=60){var h=Math.floor(m/60),mm=m%60;return h+" ч"+(mm?" "+mm+" мин":"");}
  return m+" мин";
}
function pad2(n){return ("0"+n).slice(-2);}
function localHM(ts){var d=new Date(ts*1000);return pad2(d.getHours())+":"+pad2(d.getMinutes());}
function localDateHM(ts){var d=new Date(ts*1000);return d.getFullYear()+"-"+pad2(d.getMonth()+1)+"-"+pad2(d.getDate())+" "+pad2(d.getHours())+":"+pad2(d.getMinutes());}
// время изменений статусов — в часовом поясе смотрящего (по epoch)
function fmtTime(ts,ds){return localHM(ts);}
function maintLabel(s){
  if(!s.maintenance)return "";
  var t="на обслуживании";
  if(s.maintTo&&s.maintTo>0)t+=" · до "+localHM(s.maintTo);
  return t;
}
function escapeHtml(s){var d=document.createElement("div");d.textContent=s;return d.innerHTML;}
var tip=document.getElementById("tip");
function evXY(e){var t=e.touches&&e.touches[0];return t||e;}
function moveTip(e){
  e=evXY(e);
  var w=tip.offsetWidth,x=e.clientX+14,y=e.clientY-10;
  if(x+w>window.innerWidth-8)x=e.clientX-w-14;
  tip.style.left=x+"px";tip.style.top=Math.max(8,y)+"px";
}
function hideTip(){tip.style.opacity="0";}
function showTipDay(e,d){
  var u=d.uptime;
  var uc=(u===null)?"var(--tx2)":(u>=99.99?"var(--ok)":(u>=95?"var(--warn)":"var(--bad)"));
  var ut=(u===null)?"нет данных":u.toFixed(2)+"%";
  var down=!d.hasData?''
    :d.downMin>0?'<div class="k">простой: <b style="color:var(--bad)">'+fmtDur(d.downMin)+'</b></div>'
    :((d.uptime!==null&&d.uptime<100)?'<div class="k">кратковременный сбой</div>':'<div class="k">сбоев нет</div>');
  tip.innerHTML='<div class="d">'+d.label+'</div><div class="k">аптайм: <b style="color:'+uc+'">'+ut+'</b></div>'+down;
  tip.style.opacity="1";moveTip(e);
}
function showTipServer(e,s){
  var u=s.uptime30;
  var uc=(u===null)?"var(--tx2)":(u>=99.9?"var(--ok)":(u>=99?"var(--warn)":"var(--bad)"));
  var ut=(u===null)?"нет данных":u.toFixed(2)+"%";
  var dm=(s.downMin30>0)?'<b style="color:var(--bad)">'+fmtDur(s.downMin30)+'</b>':'<b style="color:var(--ok)">0 мин</b>';
  tip.innerHTML='<div class="d">'+escapeHtml(s.name)+'</div>'+
    '<div class="k">статус: '+(s.online?'<b style="color:var(--ok)">онлайн</b>':'<b style="color:var(--bad)">офлайн</b>')+'</div>'+
    '<div class="k">аптайм 30 дн: <b style="color:'+uc+'">'+ut+'</b></div>'+
    '<div class="k">общий простой: '+dm+'</div>';
  tip.style.opacity="1";moveTip(e);
}
function renderToday(panel,data){
  var head=data.dayLabel||"Сегодня";
  if(!data.samples.length){
    panel.innerHTML='<div class="phead">'+head+'</div><div class="empty">Данных за этот день нет.</div>';
    panelH(panel);return;
  }
  var ds=data.dayStart,span=86400,W=1000;
  var chartTop=10,base=150,H=150,chartH=base-chartTop,mid=(chartTop+base)/2;
  var sw=Math.max(1.5,data.pollInterval/span*W);
  var bands="",runStart=null,runEnd=null,ptsRaw=[];
  function flush(){if(runStart!==null){bands+='<rect x="'+runStart.toFixed(1)+'" y="0" width="'+Math.max(1.5,runEnd-runStart).toFixed(1)+'" height="'+H+'" fill="rgba(232,80,80,.18)"/>';runStart=null;}}
  data.samples.forEach(function(s){
    var x=(s.ts-ds)/span*W;
    if(s.online){flush();if(s.latency>0)ptsRaw.push([x,s.latency]);}
    else{if(runStart===null)runStart=x;runEnd=x+sw;}
  });
  flush();
  function niceLo(v){var e=Math.pow(10,Math.floor(Math.log(v)/Math.LN10+1e-9)),m=v/e;return (m>=5?5:(m>=2?2:1))*e;}
  function niceHi(v){var e=Math.pow(10,Math.floor(Math.log(v)/Math.LN10+1e-9)),m=v/e;return (m<=1.0001?1:(m<=2.0001?2:(m<=5.0001?5:10)))*e;}
  var pmin=data.stats.pmin||50,pmax=data.stats.pmax||100;
  var lo=niceLo(Math.max(10,pmin)),hi=niceHi(Math.max(pmax,lo*4));
  var L0=Math.log(lo),LR=(Math.log(hi)-L0)||1;
  function yOf(v){var vv=v<lo?lo:(v>hi?hi:v);return base-(Math.log(vv)-L0)/LR*chartH;}
  var pts=ptsRaw.map(function(p){return [p[0],yOf(p[1])];});
  var ticks=[],t=lo,tg=0;
  while(t<=hi*1.0001&&tg++<40){ticks.push(t);var te=Math.pow(10,Math.floor(Math.log(t)/Math.LN10+1e-9)),tm=t/te;t=(tm<1.5?2:(tm<3.5?5:10))*te;}
  var grid="",yl="";
  ticks.forEach(function(tk){var ty=yOf(tk);grid+='<line x1="0" y1="'+ty.toFixed(1)+'" x2="1000" y2="'+ty.toFixed(1)+'" stroke="var(--line)" stroke-width="1"/>';yl+='<div class="tyaxis" style="top:'+ty.toFixed(1)+'px">'+tk+'</div>';});
  var poly=pts.map(function(p){return p[0].toFixed(1)+","+p[1].toFixed(1);});
  var line=pts.length?'<polyline points="'+poly.join(" ")+'" fill="none" stroke="#2f6bff" stroke-width="1.6" stroke-linejoin="round" stroke-linecap="round"/>':"";
  var area=pts.length?'<path d="M'+pts[0][0].toFixed(1)+','+base+' L'+poly.join(" L")+' L'+pts[pts.length-1][0].toFixed(1)+','+base+' Z" fill="url(#pgrad)"/>':"";
  var nowLine="";
  if(data.isToday){var nowX=((data.now-ds)/span*W);nowLine='<line x1="'+nowX.toFixed(1)+'" y1="0" x2="'+nowX.toFixed(1)+'" y2="'+base+'" stroke="var(--tx3)" stroke-width="1" stroke-dasharray="4 4"/>';}
  var svg='<svg viewBox="0 0 1000 '+H+'" preserveAspectRatio="none" class="tchart">'+
    '<defs><linearGradient id="pgrad" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#2f6bff" stop-opacity="0.20"/><stop offset="1" stop-color="#2f6bff" stop-opacity="0"/></linearGradient></defs>'+
    grid+area+bands+line+nowLine+
    '<line class="cursor" x1="0" y1="0" x2="0" y2="'+base+'" stroke="var(--tx2)" stroke-width="1" opacity="0"/>'+
    '</svg>';
  var zoom=2;
  var axis="";for(var h2=0;h2<=24;h2+=3){axis+='<span>'+("0"+h2).slice(-2)+':00</span>';}
  var st=data.stats,last=data.samples[data.samples.length-1];
  var stats='<div class="tstats">'+
    '<div><span>'+(data.isToday?"Проверок сегодня":"Проверок за день")+'</span><b>'+st.checks+'</b></div>'+
    '<div><span>Ошибок опроса</span><b style="color:'+(st.errors?"var(--bad)":"var(--ok)")+'">'+st.errors+'</b></div>'+
    '<div><span>'+(data.isToday?"Пинг сейчас":"Последний пинг")+'</span><b>'+(last.online?last.latency+" мс":"—")+'</b></div>'+
    '<div><span>Пинг мин / сред / макс</span><b>'+(st.pavg?st.pmin+" / "+st.pavg+" / "+st.pmax+" мс":"—")+'</b></div>'+
    '</div>';
  var cap='Пинг, мс · логарифмическая шкала · макс '+(data.stats.pmax||0)+' мс';
  panel.innerHTML='<div class="phead">'+head+' · синяя линия — пинг, красные полосы — периоды сбоев</div>'+
    stats+
    '<div class="tcaption">'+cap+'</div>'+
    '<div class="tchartwrap">'+
      '<div class="tscroll"><div class="tcanvas" style="width:'+(zoom*100)+'%">'+svg+'<div class="taxis">'+axis+'</div></div></div>'+
      yl+
    '</div>';
  var sc=panel.querySelector(".tscroll");
  if(sc){
    if(data.isToday){
      var nf=(data.now-ds)/span,tgt=nf*sc.scrollWidth-sc.clientWidth/2;
      sc.scrollLeft=Math.max(0,Math.min(tgt,sc.scrollWidth-sc.clientWidth));
    }else{sc.scrollLeft=0;}
  }
  var svgEl=panel.querySelector("svg"),cursor=svgEl.querySelector(".cursor"),samples=data.samples;
  function scrub(cx,ev){
    var r=svgEl.getBoundingClientRect();
    var frac=(cx-r.left)/r.width;if(frac<0)frac=0;if(frac>1)frac=1;
    var ts=ds+frac*span,best=null,bd=1e15;
    for(var i=0;i<samples.length;i++){var dd=Math.abs(samples[i].ts-ts);if(dd<bd){bd=dd;best=samples[i];}}
    if(!best)return;
    var x=(best.ts-ds)/span*1000;
    cursor.setAttribute("x1",x);cursor.setAttribute("x2",x);cursor.setAttribute("opacity","1");
    tip.innerHTML='<div class="d">'+fmtTime(best.ts,ds)+'</div>'+
      (best.online?'<div class="k">опрос: <b style="color:var(--ok)">успешно</b></div><div class="k">пинг: <b>'+best.latency+' мс</b></div>'
                  :'<div class="k"><b style="color:var(--bad)">ошибка опроса</b></div>');
    tip.style.opacity="1";moveTip(ev);
  }
  svgEl.addEventListener("mousemove",function(e){scrub(e.clientX,e);});
  svgEl.addEventListener("mouseleave",function(){cursor.setAttribute("opacity","0");hideTip();});
  var td=null;
  svgEl.addEventListener("touchstart",function(e){var t=e.touches[0];if(!t)return;td={x:t.clientX,y:t.clientY,m:false};scrub(t.clientX,t);},{passive:true});
  svgEl.addEventListener("touchmove",function(e){if(!td)return;var t=e.touches[0];if(!t)return;if(!td.m&&(Math.abs(t.clientX-td.x)>8||Math.abs(t.clientY-td.y)>8)){td.m=true;cursor.setAttribute("opacity","0");hideTip();}},{passive:true});
  svgEl.addEventListener("touchend",function(){td=null;},{passive:true});
  panelH(panel);
}
var built=false,order=[],nodes={};
function srvUpColor(u){return (u===null)?"var(--tx2)":(u>=99.9?"var(--ok)":(u>=99?"var(--warn)":"var(--bad)"));}
function updateTop(data){
  document.getElementById("title").textContent=data.title;
  if(data.title)document.title=data.title;
  document.getElementById("subtitle").textContent=data.subtitle;
  var t=data.totals;
  var onlineEl=document.getElementById("s-online");
  var oh=t.online+" / "+t.total;
  if(t.maintenance&&t.maintenance>0)oh+=' <span class="vsub" title="'+t.maintenance+' на обслуживании">🛠 '+t.maintenance+'</span>';
  onlineEl.innerHTML=oh;
  document.getElementById("s-uptime").textContent=(t.uptime30===null?"—":t.uptime30.toFixed(2)+"%");
  document.getElementById("s-ping").textContent=(t.avgLatency?t.avgLatency+" ms":"—");
  var fresh=document.getElementById("s-fresh");
  if(fresh)fresh.textContent=data.lastCheckTs?localHM(data.lastCheckTs):(data.lastCheck||"—");
  var allUp=t.online===t.total&&t.total>0;
  var ov=document.getElementById("overall");
  ov.className="pill "+(allUp?"ok":"bad");
  ov.innerHTML='<span class="dot" style="background:currentColor"></span><span>'+
    (allUp?"Все активные серверы доступны":(t.total-t.online)+" сервер(ов) недоступно")+'</span>';
  document.getElementById("foot").textContent=
    (data.lastCheckTs?"последняя проверка "+localDateHM(data.lastCheckTs):(data.lastCheck?"последняя проверка "+data.lastCheck:"ожидание первой проверки"))+
    (data.pollInterval?" · обновление раз в "+Math.round(data.pollInterval/60*10)/10+" мин":"");
}
function panelH(panel){
  var it=panel.parentElement;
  if(it&&it.classList.contains("open"))panel.style.maxHeight=(panel.scrollHeight+40)+"px";
}
function openPanel(panel,sid){
  hideTip();
  if(!panel.innerHTML.trim()||panel.querySelector(".empty"))panel.innerHTML='<div class="empty">Загрузка…</div>';
  fetch("api/today?sid="+encodeURIComponent(sid))
    .then(function(r){return r.json();})
    .then(function(td){renderToday(panel,td);})
    .catch(function(){panel.innerHTML='<div class="empty">Не удалось загрузить.</div>';panelH(panel);});
}
function refreshPanel(panel,sid){
  var sc=panel.querySelector(".tscroll");
  var keep=sc?sc.scrollLeft:null;
  fetch("api/today?sid="+encodeURIComponent(sid))
    .then(function(r){return r.json();})
    .then(function(td){
      renderToday(panel,td);
      if(keep!==null){var s2=panel.querySelector(".tscroll");if(s2)s2.scrollLeft=keep;}
    })
    .catch(function(){});
}
function loadDay(panel,sid,date){
  hideTip();
  if(!panel.innerHTML.trim()||panel.querySelector(".empty"))panel.innerHTML='<div class="empty">Загрузка…</div>';
  fetch("api/day?sid="+encodeURIComponent(sid)+"&date="+encodeURIComponent(date))
    .then(function(r){return r.json();})
    .then(function(td){renderToday(panel,td);})
    .catch(function(){panel.innerHTML='<div class="empty">Не удалось загрузить.</div>';panelH(panel);});
}
function applyServer(item,s,days){
  item._label._s=s;
  item._dot.style.background=s.maintenance?"#2f8fff":(s.online?"#16b07a":"#e8504e");
  for(var i=0;i<item._bars.length;i++){var d=s.days[i];if(d){item._bars[i]._d=d;item._bars[i].setAttribute("fill",colorFor(d));}}
  if(s.maintenance){
    item._p.textContent="обслуж.";
    item._p.style.color="var(--info)";
    item._s2.textContent=(s.maintTo&&s.maintTo>0)?("примерно до "+localHM(s.maintTo)):"идут работы";
  }else{
    item._p.textContent=(s.uptime30===null)?"—":s.uptime30.toFixed(2)+"%";
    item._p.style.color=srvUpColor(s.uptime30);
    item._s2.textContent=(s.latencyMs?s.latencyMs+" ms · ":"")+days+" дн";
  }
}
function buildList(data){
  var list=document.getElementById("list");
  list.innerHTML="";nodes={};order=[];
  data.servers.forEach(function(s,idx){
    var item=document.createElement("div");item.className="item"+(s.maintenance?" maint":"");item._sid=s.sid;
    item.style.animationDelay=Math.min(idx*0.04,0.5)+"s";
    var row=document.createElement("div");row.className="row";
    var flag=s.cc?'<img class="flag" src="/flags/'+s.cc+'.svg" alt="" loading="lazy">':'<span class="flag"></span>';
    var label=document.createElement("div");label.className="label";
    label.innerHTML=flag+'<div class="nm"><div class="name"><span class="sdot"></span><span>'+escapeHtml(s.name)+'</span></div>'+(s.maintenance?'<div class="mnt-line">\uD83D\uDEE0 '+maintLabel(s)+'</div>':'')+'</div>';
    label._s=s;
    label.addEventListener("mouseenter",function(e){showTipServer(e,this._s);});
    label.addEventListener("mousemove",moveTip);
    label.addEventListener("mouseleave",hideTip);
    var N=s.days.length||1;
    var bars=document.createElementNS(SVGNS,"svg");bars.setAttribute("class","bars");
    bars.setAttribute("viewBox","0 0 1000 100");bars.setAttribute("preserveAspectRatio","none");
    var slot=1000/N,gap=Math.min(7,slot*0.32),barArr=[];
    s.days.forEach(function(d,i){
      var r=document.createElementNS(SVGNS,"rect");
      r.setAttribute("x",(i*slot+gap/2).toFixed(2));r.setAttribute("y","0");
      r.setAttribute("width",(slot-gap).toFixed(2));r.setAttribute("height","100");
      r.setAttribute("rx","7");r.setAttribute("fill",colorFor(d));r._d=d;
      r.addEventListener("mouseenter",function(e){showTipDay(e,this._d);});
      r.addEventListener("mousemove",moveTip);
      r.addEventListener("mouseleave",hideTip);
      r.addEventListener("click",function(e){e.stopPropagation();item._day=this._d.date;item.classList.add("open");loadDay(item._panel,item._sid,this._d.date);});
      bars.appendChild(r);barArr.push(r);
    });
    var st2=document.createElement("div");st2.className="stat2";
    var pEl=document.createElement("div");pEl.className="p";
    var sEl=document.createElement("div");sEl.className="s";
    st2.appendChild(pEl);st2.appendChild(sEl);
    var chev=document.createElement("div");chev.className="chev";
    chev.innerHTML='<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 9l6 6 6-6"/></svg>';
    row.appendChild(label);row.appendChild(bars);row.appendChild(st2);row.appendChild(chev);
    var panel=document.createElement("div");panel.className="panel";panel.innerHTML='<div class="empty">Загрузка…</div>';
    row.addEventListener("click",function(){
      if(item.classList.contains("open")){
        panel.style.maxHeight=panel.scrollHeight+"px";
        requestAnimationFrame(function(){item.classList.remove("open");panel.style.maxHeight="0px";});
      }else{
        item._day=null;item.classList.add("open");openPanel(panel,item._sid);
      }
    });
    item.appendChild(row);item.appendChild(panel);
    item._dot=label.querySelector(".sdot");item._bars=barArr;item._p=pEl;item._s2=sEl;item._label=label;item._panel=panel;
    applyServer(item,s,data.days);
    list.appendChild(item);order.push(skey(s));nodes[s.sid]=item;
  });
  built=true;
}
function skey(s){return s.sid+"|"+(s.hidden?1:0)+(s.absent?1:0)+(s.maintenance?1:0);}
function sameOrder(data){
  if(!built||order.length!==data.servers.length)return false;
  for(var i=0;i<order.length;i++)if(order[i]!==skey(data.servers[i]))return false;
  return true;
}
var lastSeen=null;
function renderIncidents(data){
  var el=document.getElementById("incidents");if(!el)return;
  var incs=(data&&data.incidents)||[];
  if(!incs.length){el.innerHTML="";el.style.display="none";return;}
  el.style.display="";
  var sev={minor:"Незначительный",major:"Серьёзный",critical:"Критический"};
  var stt={investigating:"Расследуем",identified:"Причина найдена",monitoring:"Наблюдаем",resolved:"Устранён"};
  var html="";
  incs.forEach(function(i){var sv=i.severity||"minor";
    var started=i.startedTs?'<span class="inc-time">🕒 начат '+localHM(i.startedTs)+'</span>':'';
    var aff='';
    if(i.affected&&i.affected.length){
      var ap=i.affected.map(function(a){
        if(typeof a==="string"){return escapeHtml(a);}
        var f=a.cc?'<img class="flag" src="/flags/'+a.cc+'.svg" alt="" loading="lazy">':'';
        return f+escapeHtml(a.name||"");
      });
      aff='<div class="inc-aff">🎯 затронуты: '+ap.join(", ")+'</div>';
    }
    var tl='';
    if(i.updates&&i.updates.length){
      tl='<div class="inc-tl">';
      i.updates.forEach(function(u){
        var body=(u.body&&u.body!=="статус обновлён")?' — '+escapeHtml(u.body):'';
        tl+='<div class="inc-tlrow"><span class="inc-tlt">'+localHM(u.ts)+'</span><span class="inc-tls">'+escapeHtml(stt[u.status]||u.status||'')+'</span>'+body+'</div>';
      });
      tl+='</div>';
    }
    html+='<div class="inc-card inc-'+sv+'"><div class="inc-h"><span class="inc-sev">'+escapeHtml(sev[sv]||sv)+'</span><span class="inc-status">'+escapeHtml(stt[i.status]||i.status||"")+'</span>'+started+'</div><div class="inc-title">'+escapeHtml(i.title||"")+'</div>'+aff+tl+'</div>';});
  el.innerHTML=html;
}
function render(data){
  updateTop(data);
  renderIncidents(data);
  var fresh=data.lastCheck!==lastSeen;lastSeen=data.lastCheck;
  if(sameOrder(data)){
    data.servers.forEach(function(s){var item=nodes[s.sid];if(item)applyServer(item,s,data.days);});
    if(fresh)for(var sid in nodes){var it=nodes[sid];if(it.classList.contains("open")&&(it._day===null||it._day===undefined))refreshPanel(it._panel,it._sid);}
  }else{
    buildList(data);
  }
}
function load(){
  fetch("api/summary").then(function(r){
    if(!r.ok)throw new Error("http "+r.status);
    return r.json();
  }).then(function(d){
    if(!d||!d.servers)throw new Error("bad payload");
    render(d);
  })
  .catch(function(){
    // Разовый сбой опроса (рестарт/таймаут) НЕ должен стирать уже показанные
    // серверы. Сбрасываем built, чтобы при след. успехе была полная пересборка,
    // и показываем сообщение только если на странице ещё ничего нет.
    built=false;
    var list=document.getElementById("list");
    if(!list.querySelector(".item")){
      list.innerHTML='<div class="skel">Обновление не удалось — пробуем снова…</div>';
    }
  });
}
window.addEventListener("resize",function(){
  var ps=document.querySelectorAll(".item.open .panel");
  for(var i=0;i<ps.length;i++)ps[i].style.maxHeight=ps[i].scrollHeight+"px";
});
load();
setInterval(function(){if(!document.hidden)load();},60000);
document.addEventListener("visibilitychange",function(){if(!document.hidden)load();});
</script>
</body>
</html>
