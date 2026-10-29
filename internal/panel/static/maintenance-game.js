(() => {
    const board=document.getElementById('game-board'),result=document.getElementById('game-result'),keep=document.getElementById('continue-game');
    const key='router-2048-v1';
    const cells=Array.from({length:16},()=>{const el=document.createElement('div');el.className='game-cell';el.setAttribute('role','img');board.append(el);return el;});
    class Storage {
        getBestScore(){try{return Number(localStorage.getItem(key+':best'))||0}catch(_){return 0}}
        setBestScore(value){try{localStorage.setItem(key+':best',value)}catch(_){}}
        getGameState(){try{
            const state=JSON.parse(localStorage.getItem(key+':state'));
            if(!state||state.grid.size!==4||!Number.isFinite(state.score)||state.score<0)return null;
            if(state.grid.cells.length!==4||!state.grid.cells.every((col,x)=>col.length===4&&col.every((tile,y)=>!tile||tile.position.x===x&&tile.position.y===y&&Number.isInteger(Math.log2(tile.value))&&tile.value>=2&&tile.value<=131072)))return null;
            return state;
        }catch(_){return null}}
        setGameState(value){try{localStorage.setItem(key+':state',JSON.stringify(value))}catch(_){}}
        clearGameState(){try{localStorage.removeItem(key+':state')}catch(_){}}
    }
    class Input {
        constructor(){
            this.events={};
            board.addEventListener('keydown',event=>{const map={ArrowUp:0,ArrowRight:1,ArrowDown:2,ArrowLeft:3};if(event.key in map&&!event.ctrlKey&&!event.metaKey&&!event.altKey){event.preventDefault();this.emit('move',map[event.key]);}});
            document.querySelectorAll('[data-move]').forEach(button=>button.onclick=()=>this.emit('move',Number(button.dataset.move)));
            document.getElementById('restart-game').onclick=()=>this.emit('restart');keep.onclick=()=>this.emit('keepPlaying');
            let start=null;
            board.addEventListener('pointerdown',event=>{if(!event.isPrimary)return;start={x:event.clientX,y:event.clientY,id:event.pointerId};board.setPointerCapture(event.pointerId);board.focus({preventScroll:true});});
            board.addEventListener('pointerup',event=>{if(!start||event.pointerId!==start.id)return;const dx=event.clientX-start.x,dy=event.clientY-start.y;start=null;if(Math.max(Math.abs(dx),Math.abs(dy))<18)return;this.emit('move',Math.abs(dx)>Math.abs(dy)?dx>0?1:3:dy>0?2:0);});
            board.addEventListener('pointercancel',()=>start=null);
        }
        on(event,callback){this.events[event]=callback}
        emit(event,value){this.events[event]?.(value)}
    }
    class Renderer {
        actuate(grid,metadata){
            for(let y=0;y<4;y++)for(let x=0;x<4;x++){
                const tile=grid.cells[x][y],cell=cells[y*4+x],value=tile?.value||0;
                cell.textContent=value||'';cell.dataset.value=value;cell.dataset.large=String(value>=128);cell.style.fontSize=value>=10000?'18px':'';
                cell.setAttribute('aria-label',`Ряд ${y+1}, столбец ${x+1}: ${value||'пусто'}`);
                cell.classList.toggle('is-new',!!tile&&!tile.previousPosition);
            }
            document.getElementById('game-score').textContent=metadata.score;document.getElementById('game-best').textContent=metadata.bestScore;
            result.textContent=metadata.over?'Ходов больше нет. Счёт: '+metadata.score:metadata.terminated&&metadata.won?'2048! Отличная партия.':'';
            keep.hidden=!(metadata.terminated&&metadata.won);
        }
        continueGame(){result.textContent='';keep.hidden=true}
    }
    new GameManager(4,Input,Renderer,Storage);
})();
