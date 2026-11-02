
(function(){
 function setStatus(t,e){var el=document.getElementById('status');el.textContent=t||'';el.style.color=e?'var(--accent-red)':'var(--text-secondary)';}
 fetch('/account/data',{credentials:'same-origin',headers:{Accept:'application/json'}}).then(function(r){if(!r.ok) throw new Error('Не удалось загрузить аккаунт'); return r.json();}).then(function(d){document.getElementById('username').value=d.username||'';document.getElementById('account-name').textContent=d.username||'Аккаунт'; ['username','password','save-btn'].forEach(id=>document.getElementById(id).disabled=false);setStatus('');}).catch(e=>setStatus(window.RouterUI.errorMessage(e),true));
 document.getElementById('account-form').addEventListener('submit', function(event){
  event.preventDefault();
  document.getElementById('save-btn').disabled=true;
  document.getElementById('save-btn').textContent='Сохранение…';
  var b=new URLSearchParams(); b.set('username',document.getElementById('username').value.trim()); b.set('password',document.getElementById('password').value);
  fetch('/account/config',{method:'POST',credentials:'same-origin',headers:{'Content-Type':'application/x-www-form-urlencoded'},body:b.toString()})
   .then(function(r){if(!r.ok) throw new Error('Не удалось сохранить аккаунт. Проверьте логин и пароль.'); return r.json();})
   .then(function(){document.getElementById('password').value=''; document.getElementById('account-name').textContent=document.getElementById('username').value; setStatus('Изменения сохранены',false);})
   .catch(function(e){setStatus(window.RouterUI.errorMessage(e),true)})
   .finally(function(){document.getElementById('save-btn').disabled=false;document.getElementById('save-btn').textContent='Сохранить изменения';});
 });
 document.getElementById('logout-btn').addEventListener('click', function(){
   this.disabled=true;
   fetch('/logout',{method:'POST',credentials:'same-origin'}).then(function(r){if(!r.ok) throw new Error('Не удалось выйти. Повторите попытку.');location.href='/login';}).catch(function(e){setStatus(window.RouterUI.errorMessage(e),true);document.getElementById('logout-btn').disabled=false;});
 });
})();
