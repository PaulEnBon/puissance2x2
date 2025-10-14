(function(){
  // app.js - decorative animated background for the Puissance 4 page
  // Self-contained: no external deps, draws procedural textured background and subtle particles.
  'use strict';

  const canvas = document.getElementById('bgCanvas');
  if (!canvas) return;

  const dpr = Math.max(1, window.devicePixelRatio || 1);
  const ctx = canvas.getContext('2d');

  let w = 0, h = 0;
  let raf = null;
  let paused = false;
  let then = performance.now();

  const settings = {
    particleCount: 36,
    particleSize: 2.8,
    speed: 0.06,
    noiseScale: 0.0025,
    textureAlpha: 0.03
  };

  // Simple seeded pseudo-random for repeatability
  function createRng(seed){
    let s = seed >>> 0;
    return function(){
      s = (s + 0x6D2B79F5) | 0;
      let t = Math.imul(s ^ (s >>> 15), 1 | s);
      t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
  }

  const rng = createRng(1234567);

  // Particles
  const particles = [];
  function resetParticles(){
    particles.length = 0;
    for(let i=0;i<settings.particleCount;i++){
      particles.push({
        x: rng(),
        y: rng(),
        vx: (rng()-0.5)*0.003,
        vy: (rng()-0.5)*0.003,
        r: (0.6 + rng()*1.6) * settings.particleSize,
        hue: Math.floor(200 + rng()*80),
        alpha: 0.2 + rng()*0.6
      });
    }
  }

  // Resize handling
  function resize(){
    w = Math.max(300, window.innerWidth);
    h = Math.max(200, window.innerHeight);
    canvas.width = Math.round(w * dpr);
    canvas.height = Math.round(h * dpr);
    canvas.style.width = w + 'px';
    canvas.style.height = h + 'px';
    ctx.setTransform(dpr,0,0,dpr,0,0);
  }

  // Procedural noise (fast, value-noise lookup)
  function noise(x,y){
    // simple hash
    const xi = Math.floor(x)|0;
    const yi = Math.floor(y)|0;
    const h = Math.sin(xi*127.1 + yi*311.7) * 43758.5453123;
    return h - Math.floor(h);
  }

  // Subtle textured background rendered into a low-res offscreen canvas
  // to avoid per-pixel speckles when scaled up.
  let off = null;
  let offW = 0, offH = 0;
  function ensureOffscreen(){
    // choose a small scale for texture (keeps noise but smooth when scaled)
    const scale = 0.25; // 25% resolution
    offW = Math.max(64, Math.round(w * scale));
    offH = Math.max(48, Math.round(h * scale));
    if(!off) off = document.createElement('canvas');
    if(off.width !== offW || off.height !== offH){
      off.width = offW;
      off.height = offH;
      off.getContext('2d').imageSmoothingEnabled = true;
    }
  }

  function drawTexture(){
    ensureOffscreen();
    const octx = off.getContext('2d');
    const img = octx.createImageData(offW, offH);
    const data = img.data;
    const nscale = settings.noiseScale * 4.0; // sample coarser on the small canvas
    for(let j=0;j<offH;j++){
      for(let i=0;i<offW;i++){
        const t = noise(i * nscale, j * nscale);
        const v = Math.floor(t * 255);
        const idx = (j*offW + i) * 4;
        data[idx]   = 18 + v * 0.12; // r
        data[idx+1] = 30 + v * 0.18; // g
        data[idx+2] = 58 + v * 0.28; // b
        data[idx+3] = 255; // fully opaque in offscreen canvas
      }
    }
    octx.putImageData(img, 0, 0);
    // draw scaled up with smoothing and slight blur for a soft texture
    ctx.save();
    ctx.globalAlpha = settings.textureAlpha;
    // apply a tiny blur when supported (keeps texture soft)
    try{ ctx.filter = 'blur(1.5px)'; } catch(e){}
    ctx.drawImage(off, 0, 0, w, h);
    try{ ctx.filter = 'none'; } catch(e){}
    ctx.restore();
  }

  // Draw radial vignette
  function drawVignette(){
    const grad = ctx.createRadialGradient(w/2, h/2, Math.min(w,h)*0.1, w/2, h/2, Math.max(w,h)*0.8);
    grad.addColorStop(0, 'rgba(0,0,0,0)');
    grad.addColorStop(1, 'rgba(0,0,0,0.22)');
    ctx.fillStyle = grad;
    ctx.fillRect(0,0,w,h);
  }

  // Draw particles
  function drawParticles(dt){
    for(const p of particles){
      // Flow field - use noise to nudge velocity
      const nx = noise(p.x*500, p.y*500 + then*0.0001);
      const ny = noise(p.x*400 + 99, p.y*400 + then*0.00023);
      p.vx += (nx - 0.5) * settings.speed * dt;
      p.vy += (ny - 0.5) * settings.speed * dt;

      p.x += p.vx * dt * 60;
      p.y += p.vy * dt * 60;

      // wrap
      if(p.x < -0.1) p.x = 1.1;
      if(p.x > 1.1) p.x = -0.1;
      if(p.y < -0.1) p.y = 1.1;
      if(p.y > 1.1) p.y = -0.1;

      const X = p.x * w;
      const Y = p.y * h;
      const g = ctx.createRadialGradient(X,Y,p.r*0.1,X,Y,p.r);
      g.addColorStop(0, `hsla(${p.hue}, 85%, 60%, ${p.alpha})`);
      g.addColorStop(1, `hsla(${p.hue}, 70%, 40%, 0)`);
      ctx.fillStyle = g;
      ctx.beginPath();
      ctx.arc(X,Y,p.r,0,Math.PI*2);
      ctx.fill();
    }
  }

  function clear(){
    ctx.clearRect(0,0,w,h);
  }

  function frame(now){
    try {
      if (paused) return;
      const dt = Math.min(64, now - then);
      then = now;

      clear();

      // Draw tiled subtle texture once per second-ish - but keep cheap
      // We'll composite with multiply to tint the page
      ctx.save();
      ctx.globalCompositeOperation = 'screen';
      drawTexture();
      ctx.restore();

      // Particles overlay
      ctx.save();
      ctx.globalCompositeOperation = 'lighter';
      drawParticles(dt/16);
      ctx.restore();

      // Vignette to anchor center
      drawVignette();
    } catch (err) {
      // Log and swallow to avoid crashing the whole script
      try { console.error('bgAnimation frame error:', err); } catch(e){}
    } finally {
      raf = requestAnimationFrame(frame);
    }
  }

  // public controls
  window.__bgAnimation = {
    pause(){ paused = true; if(raf) cancelAnimationFrame(raf); raf = null; },
    resume(){ if(!raf){ paused = false; then = performance.now(); raf = requestAnimationFrame(frame); } },
    toggle(){ paused = !paused; if(paused) this.pause(); else this.resume(); },
    settings
  };

  // Effects toggle: persists in localStorage
  const EFFECTS_KEY = 'puissance_effects_on';
  function effectsEnabled(){
    const v = localStorage.getItem(EFFECTS_KEY);
    if(v === null) return true; // default on
    return v === '1';
  }
  function setEffectsEnabled(on){
    localStorage.setItem(EFFECTS_KEY, on ? '1' : '0');
    // apply
    if(on){ window.__bgAnimation.resume(); } else { window.__bgAnimation.pause(); }
  }
  function wireEffectsToggle(){
    const btn = document.getElementById('effectsToggle');
    if(!btn) return;
    const on = effectsEnabled();
    btn.setAttribute('aria-pressed', on ? 'true' : 'false');
    btn.addEventListener('click', ()=>{
      const newOn = !effectsEnabled();
      btn.setAttribute('aria-pressed', newOn ? 'true' : 'false');
      setEffectsEnabled(newOn);
    });
    // initial
    setEffectsEnabled(on);
  }

  // ------------ UI animations: token drop and confetti ------------
  // Create floating token element for drop animation
  function createFloatingToken(color){
    const el = document.createElement('div');
    el.className = 'floating-token ' + color;
    // style fallback if CSS missing - adjusted for larger size
    el.style.position = 'absolute';
    el.style.width = '48px';
    el.style.height = '48px';
    el.style.borderRadius = '50%';
    el.style.pointerEvents = 'none';
    el.style.zIndex = '1000';
    return el;
  }

  // Animate dropping token from button to target cell (visual only, doesn't block submit)
  function wireDropAnimations(){
    const buttons = document.querySelectorAll('.col-button');
    if(!buttons || buttons.length===0) return;
    buttons.forEach(btn => {
      btn.addEventListener('click', function(ev){
        try{
          // Visual-only animation: don't prevent default, let form submit naturally
          const col = btn.getAttribute('data-column');
          console.debug('[bg] col-button click, data-column=', col);
          
          const colIndex = Number(col);
          if(Number.isNaN(colIndex) || colIndex < 0) return;

          // determine target cell: find first empty cell from bottom in that column
          const table = document.querySelector('.board');
          if(!table) return;
          const tb = table.tBodies && table.tBodies[0];
          if(!tb) return;
          const rows = Array.from(tb.rows || []);
          let targetCell = null;
          
          for(let i = rows.length-1; i>=0; i--){
            const td = rows[i].cells[colIndex];
            if(!td) continue;
            const span = td.querySelector('.cell');
            if(span && !span.classList.contains('R') && !span.classList.contains('Y')){
              targetCell = td;
              break;
            }
          }
          
          if(!targetCell) return; // column full, no animation
          
          // create token at button position
          const color = (document.getElementById('status') && document.getElementById('status').textContent.indexOf('Au tour de: R')!==-1) ? 'R' : 'Y';
          const token = createFloatingToken(color);
          document.body.appendChild(token);

          const btnRect = btn.getBoundingClientRect();
          token.style.left = (btnRect.left + btnRect.width/2 - 24) + 'px';
          token.style.top = (btnRect.top + btnRect.height/2 - 24) + 'px';

          const cellRect = targetCell.getBoundingClientRect();
          const destX = cellRect.left + cellRect.width/2 - 24;
          const destY = cellRect.top + cellRect.height/2 - 24;

          // animate using requestAnimationFrame (visual only, page will reload)
          const start = performance.now();
          const duration = 400; // slightly faster since page reloads
          const sx = parseFloat(token.style.left) || 0;
          const sy = parseFloat(token.style.top) || 0;
          
          function animate(now){
            const t = Math.min(1, (now - start)/duration);
            const ease = 1 - Math.pow(1 - t, 3); // easeOutCubic
            token.style.left = (sx + (destX - sx) * ease) + 'px';
            token.style.top = (sy + (destY - sy) * ease) + 'px';
            token.style.transform = 'scale(' + (1 - 0.2 * ease) + ')';
            if(t < 1){
              requestAnimationFrame(animate);
            } else {
              try{ token.remove(); } catch(e){}
            }
          }
          requestAnimationFrame(animate);
        } catch(err){
          console.error('[bg] animation error:', err);
        }
      });
    });
  }

  // Explosion vidéo en plein écran au lieu de confettis
  function explosionVideo(){
    console.debug('[bg] explosionVideo start');
    
    // Créer l'overlay vidéo
    const overlay = document.createElement('div');
    overlay.style.position = 'fixed';
    overlay.style.top = '0';
    overlay.style.left = '0';
    overlay.style.width = '100vw';
    overlay.style.height = '100vh';
    overlay.style.backgroundColor = 'rgba(0, 0, 0, 0.7)';
    overlay.style.zIndex = '9999';
    overlay.style.display = 'flex';
    overlay.style.alignItems = 'center';
    overlay.style.justifyContent = 'center';
    overlay.style.pointerEvents = 'none';
    
    // Créer la vidéo
    const video = document.createElement('video');
    video.src = '/images/effet_explosion.mp4';
    video.autoplay = true;
    video.muted = false; // Avec son
    video.style.maxWidth = '80%';
    video.style.maxHeight = '80%';
    video.style.objectFit = 'contain';
    
    overlay.appendChild(video);
    document.body.appendChild(overlay);
    
    // Retirer l'overlay après la fin de la vidéo
    video.addEventListener('ended', () => {
      setTimeout(() => {
        overlay.style.transition = 'opacity 0.5s';
        overlay.style.opacity = '0';
        setTimeout(() => {
          if(overlay.parentNode) overlay.parentNode.removeChild(overlay);
        }, 500);
      }, 300);
    });
    
    // Fallback: retirer après 5 secondes max
    setTimeout(() => {
      if(overlay.parentNode){
        overlay.style.transition = 'opacity 0.5s';
        overlay.style.opacity = '0';
        setTimeout(() => {
          if(overlay.parentNode) overlay.parentNode.removeChild(overlay);
        }, 500);
      }
    }, 5000);
  }

  // Detect winner on load and fire explosion video
  function detectWinner(){
    const st = document.getElementById('status');
    if(!st) return;
    const txt = st.textContent || '';
    if(txt.indexOf('Gagnant') !== -1 || txt.indexOf('Gagnant:') !== -1){
      // small delay to let page settle
      setTimeout(()=> explosionVideo(), 200);
    }
  }

  // bind after DOM ready
  if(document.readyState === 'loading'){
    document.addEventListener('DOMContentLoaded', ()=>{ wireDropAnimations(); detectWinner(); wireEffectsToggle(); });
  } else {
    wireDropAnimations(); detectWinner(); wireEffectsToggle();
  }

  // boot
  function boot(){
    resize();
    resetParticles();
    then = performance.now();
    if(!raf) raf = requestAnimationFrame(frame);
  }

  window.addEventListener('resize', ()=>{
    resize();
  });

  // Respect reduced motion preference
  const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
  if(mq && mq.matches){
    paused = true;
  }

  // global error handler: log and disable effects after repeated errors
  (function(){
    let errorCount = 0;
    window.addEventListener('error', function(e){
      try{ console.error('Global error:', e.error || e.message || e); } catch(err){}
      errorCount++;
      if(errorCount > 3){
        try{ setEffectsEnabled(false); console.warn('Effects disabled due to repeated errors'); } catch(err){}
        try{ console.debug('[bg] effects auto-disabled after errors'); } catch(e){}
      }
    });
    window.addEventListener('unhandledrejection', function(e){
      try{ console.error('UnhandledRejection:', e.reason); } catch(err){}
      errorCount++;
      if(errorCount > 3){
        try{ setEffectsEnabled(false); console.warn('Effects disabled due to repeated errors'); } catch(err){}
      }
    });
  })();

  // start after DOM ready
  if(document.readyState === 'loading'){
    document.addEventListener('DOMContentLoaded', boot);
  } else {
    boot();
  }
})();
