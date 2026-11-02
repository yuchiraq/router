(() => {
    const canvas=document.getElementById('flow-field'),ctx=canvas.getContext('2d',{alpha:false}),button=document.getElementById('motion-button');
    if(!ctx){button.hidden=true;return;}
    const reduced=matchMedia('(prefers-reduced-motion:reduce)');
    let width=0,height=0,ratio=1,phase=0,last=0,frame=0,paused=reduced.matches,point={x:0,y:0},target={x:0,y:0};
    function draw(){
        const light=document.documentElement.dataset.theme==='light';
        ctx.fillStyle=light?'#eff2f0':'#101213';ctx.fillRect(0,0,width,height);
        const mobile=width<700,lines=mobile?66:100,steps=mobile?66:100;
        const cx=width*(mobile?.62:.74)+point.x*22,cy=height*(mobile?.28:.49)+point.y*20;
        const scale=Math.min(width*(mobile?.9:.48),height*.66);
        ctx.lineWidth=mobile?.75:.8;
        for(let i=0;i<lines;i++){
            const v=i/(lines-1),angle=v*Math.PI*2;
            // A folded toroidal cross-section produces continuous, interference-like contours.
            ctx.beginPath();
            for(let j=0;j<=steps;j++){
                const u=j/steps*Math.PI*2;
                const twist=.38*Math.sin(u*2+phase*.24)+.2*Math.cos(angle+phase*.17);
                const ring=.68+.25*Math.cos(angle+twist);
                const x=(Math.cos(u)*ring+.2*Math.sin(angle)*Math.sin(phase*.12))*scale;
                const y=(Math.sin(u)*ring*.64+.32*Math.sin(angle+twist))*scale;
                const turn=-.48+.12*Math.sin(phase*.11);
                const px=cx+x*Math.cos(turn)-y*Math.sin(turn),py=cy+x*Math.sin(turn)+y*Math.cos(turn);
                if(j===0)ctx.moveTo(px,py);else ctx.lineTo(px,py);
            }
            const tone=Math.sin(angle+phase*.2),alpha=light?.23:.36;
            ctx.strokeStyle=light?(tone>.35?`rgba(38,112,87,${alpha})`:`rgba(100,85,112,${alpha})`):(tone>.35?`rgba(135,226,186,${alpha})`:tone<-.4?`rgba(232,157,139,${alpha})`:`rgba(171,183,219,${alpha})`);
            ctx.stroke();
        }
    }
    function tick(now){frame=0;if(paused||document.hidden)return;if(now-last>=32){phase+=Math.min((now-last)/1000,.05);last=now;point.x+=(target.x-point.x)*.025;point.y+=(target.y-point.y)*.025;draw();}frame=requestAnimationFrame(tick);}
    function sync(){cancelAnimationFrame(frame);frame=0;button.setAttribute('aria-pressed',String(paused));button.title=button.ariaLabel=paused?'Продолжить движение':'Остановить движение';if(!paused&&!document.hidden){last=performance.now();frame=requestAnimationFrame(tick);}draw();}
    function resize(){width=innerWidth;height=innerHeight;ratio=Math.min(devicePixelRatio||1,1.5);canvas.width=Math.round(width*ratio);canvas.height=Math.round(height*ratio);ctx.setTransform(ratio,0,0,ratio,0,0);draw();}
    button.onclick=()=>{paused=!paused;sync();};
    reduced.addEventListener('change',()=>{paused=reduced.matches;sync();});
    document.addEventListener('visibilitychange',sync);addEventListener('resize',resize);
    addEventListener('pointermove',event=>{target={x:event.clientX/width-.5,y:event.clientY/height-.5};},{passive:true});
    new MutationObserver(draw).observe(document.documentElement,{attributes:true,attributeFilter:['data-theme']});
    resize();sync();
})();
