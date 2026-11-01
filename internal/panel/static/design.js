(() => {
    const reduced=()=>matchMedia('(prefers-reduced-motion:reduce)').matches;
    function moveIndicator(container,target,indicator) {
        if(!target||!target.offsetHeight){indicator.hidden=true;return;}
        indicator.hidden=false;
        indicator.style.width=target.offsetWidth+'px';indicator.style.height=target.offsetHeight+'px';
        indicator.style.transform=`translate(${target.offsetLeft}px,${target.offsetTop}px)`;
    }
    const nav=document.querySelector('.nav-links');
    if(nav){
        const marker=document.createElement('span');marker.className='nav-indicator';marker.setAttribute('aria-hidden','true');nav.prepend(marker);
        const active=()=>[...nav.querySelectorAll('.active')].find(el=>el.offsetHeight);
        const reset=()=>moveIndicator(nav,active(),marker);
        nav.querySelectorAll('a,button').forEach(el=>{el.addEventListener('pointerenter',()=>moveIndicator(nav,el,marker));el.addEventListener('focus',()=>moveIndicator(nav,el,marker));});
        nav.addEventListener('pointerleave',reset);nav.addEventListener('focusout',reset);new ResizeObserver(reset).observe(nav);reset();
    }
    document.querySelectorAll('.fluid-tabs').forEach(tabs=>{
        let marker=tabs.querySelector('.tab-indicator');
        if(!marker){marker=document.createElement('span');marker.className='tab-indicator';marker.setAttribute('aria-hidden','true');tabs.prepend(marker);}
        const update=()=>moveIndicator(tabs,tabs.querySelector('[aria-selected="true"]'),marker);
        new ResizeObserver(update).observe(tabs);window.addEventListener('router-tabs-change',update);update();
    });
    document.addEventListener('click',event=>{
        const button=event.target.closest('.btn');if(!button||button.disabled||reduced())return;
        const rect=button.getBoundingClientRect(),size=Math.max(rect.width,rect.height)*2;
        const ripple=document.createElement('span');ripple.className='button-ripple';ripple.setAttribute('aria-hidden','true');
        ripple.style.width=ripple.style.height=size+'px';ripple.style.left=((event.detail?event.clientX-rect.left:rect.width/2)-size/2)+'px';ripple.style.top=((event.detail?event.clientY-rect.top:rect.height/2)-size/2)+'px';
        button.append(ripple);setTimeout(()=>ripple.remove(),550);
    });
    if(!reduced())document.querySelectorAll('.rule-item,.settings-section').forEach((el,i)=>el.animate([{opacity:0,transform:'translateY(6px)'},{opacity:1,transform:'translateY(0)'}],{duration:280,delay:Math.min(i,6)*35,fill:'backwards'}));
    document.querySelectorAll('[data-cycle-theme]').forEach(button=>button.addEventListener('click',()=>{
        const dark=document.documentElement.dataset.theme==='dark'||!document.documentElement.dataset.theme&&matchMedia('(prefers-color-scheme:dark)').matches;
        const theme=dark?'light':'dark';document.documentElement.dataset.theme=theme;try{localStorage.setItem('router-theme',theme)}catch(_){}window.dispatchEvent(new Event('router-theme-change'));
    }));
    if(document.body.classList.contains('auth-page')){
        const grid=document.createElement('div');grid.className='interactive-grid';grid.setAttribute('aria-hidden','true');document.body.prepend(grid);
        const cells=Array.from({length:16},()=>{const el=document.createElement('span');el.className='grid-light';grid.append(el);return el;});
        function pulse(){if(document.hidden||reduced())return;const cell=cells[Math.floor(Math.random()*cells.length)];cell.style.left=Math.floor(Math.random()*innerWidth/52)*52+'px';cell.style.top=Math.floor(Math.random()*innerHeight/52)*52+'px';cell.classList.add('is-lit');setTimeout(()=>cell.classList.remove('is-lit'),2200);}
        setInterval(pulse,650);document.addEventListener('pointermove',event=>{if(reduced())return;const cell=cells[0];cell.style.left=Math.floor(event.clientX/52)*52+'px';cell.style.top=Math.floor(event.clientY/52)*52+'px';cell.classList.add('is-lit');});
    }
})();
