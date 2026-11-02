
(function(){
  function setStatus(t){document.getElementById('status').textContent=t||''}
  document.getElementById('login-form').addEventListener('submit', function(event){
    event.preventDefault();
    var button = document.getElementById('login-btn');
    if (button.disabled) return;
    button.disabled = true; button.textContent = 'Вход…'; setStatus('');
    var b=new URLSearchParams();
    b.set('username', document.getElementById('username').value.trim());
    b.set('password', document.getElementById('password').value);
    fetch('/login',{method:'POST', credentials:'same-origin', headers:{'Content-Type':'application/x-www-form-urlencoded'}, body:b.toString()})
      .then(function(r){ if(!r.ok) throw new Error(r.status === 429 ? 'Слишком много попыток. Попробуйте позже.' : r.status >= 500 ? 'Сервер недоступен. Попробуйте позже.' : 'Неверный логин или пароль'); location.href='/'; })
      .catch(function(e){ setStatus(e instanceof TypeError ? 'Нет связи с сервером' : e.message); button.disabled=false; button.textContent='Войти'; });
  });
})();
