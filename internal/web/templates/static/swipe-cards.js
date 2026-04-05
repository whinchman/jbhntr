(function () {
  'use strict';

  const COMMIT_DIST     = 100;   // px — absolute threshold
  const COMMIT_RATIO    = 0.35;  // fraction of card width
  const COMMIT_VELOCITY = 0.4;   // px/ms
  const OVERLAY_FULL    = 80;    // px drag for full overlay opacity

  // Per-gesture state (reset on each pointerdown)
  var _card       = null;
  var _startX     = 0;
  var _startY     = 0;
  var _lastX      = 0;
  var _lastTime   = 0;
  var _velocity   = 0;
  var _dragging   = false;

  // ---------------------------------------------------------------------------
  // initDeck — attach pointer listeners to the active card (if any)
  // ---------------------------------------------------------------------------
  function initDeck() {
    var card = document.querySelector('.job-card-active');
    if (!card) return;

    card.addEventListener('pointerdown',   onPointerDown);
    card.addEventListener('pointermove',   onPointerMove);
    card.addEventListener('pointerup',     onPointerUp);
    card.addEventListener('pointercancel', onPointerCancel);

    // Expand-summary button
    var expandBtn = card.querySelector('[data-expand-summary]');
    if (expandBtn) {
      expandBtn.addEventListener('click', function () {
        expandSummary(card);
      });
    }
  }

  // ---------------------------------------------------------------------------
  // Pointer event handlers
  // ---------------------------------------------------------------------------
  function onPointerDown(e) {
    if (e.button !== 0 && e.pointerType === 'mouse') return; // left button only for mouse
    _card     = e.currentTarget;
    _startX   = e.clientX;
    _startY   = e.clientY;
    _lastX    = e.clientX;
    _lastTime = e.timeStamp;
    _velocity = 0;
    _dragging = false;

    _card.setPointerCapture(e.pointerId);
    _card.style.transition = 'none';
  }

  function onPointerMove(e) {
    if (!_card) return;

    var dx      = e.clientX - _startX;
    var dy      = e.clientY - _startY;

    // Only start dragging after a horizontal movement threshold
    if (!_dragging) {
      if (Math.abs(dx) < 5) return;
      // If mostly vertical don't initiate a swipe
      if (Math.abs(dy) > Math.abs(dx) * 1.5) {
        _card = null;
        return;
      }
      _dragging = true;
    }

    // Velocity (exponential smoothing)
    var dt = e.timeStamp - _lastTime;
    if (dt > 0) {
      var rawVel = (e.clientX - _lastX) / dt;
      _velocity  = _velocity * 0.7 + rawVel * 0.3;
    }
    _lastX    = e.clientX;
    _lastTime = e.timeStamp;

    // Apply transform
    var rotate = dx * 0.05;
    _card.style.transform = 'translateX(' + dx + 'px) rotate(' + rotate + 'deg)';

    // Overlay opacity
    var ratio   = Math.min(1, Math.abs(dx) / OVERLAY_FULL);
    var approveOverlay = _card.querySelector('.job-card-overlay-approve');
    var rejectOverlay  = _card.querySelector('.job-card-overlay-reject');
    if (approveOverlay) approveOverlay.style.opacity = dx > 0 ? ratio : 0;
    if (rejectOverlay)  rejectOverlay.style.opacity  = dx < 0 ? ratio : 0;
  }

  function onPointerUp(e) {
    if (!_card || !_dragging) {
      _card     = null;
      _dragging = false;
      return;
    }

    var card = _card;
    _card    = null;
    _dragging = false;

    var dx        = e.clientX - _startX;
    var cardWidth = card.offsetWidth;
    var absDx     = Math.abs(dx);
    var absVel    = Math.abs(_velocity);

    var committed = (absDx >= COMMIT_DIST) ||
                    (cardWidth > 0 && absDx / cardWidth >= COMMIT_RATIO) ||
                    (absVel >= COMMIT_VELOCITY);

    if (committed) {
      var direction = dx > 0 ? 'approve' : 'reject';
      commitCard(card, direction);
    } else {
      snapBack(card);
    }
  }

  function onPointerCancel(e) {
    if (!_card) return;
    var card = _card;
    _card    = null;
    _dragging = false;
    snapBack(card);
  }

  // ---------------------------------------------------------------------------
  // commitCard — fly-off animation then submit
  // ---------------------------------------------------------------------------
  function commitCard(card, direction) {
    var reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (reducedMotion) {
      submitAction(direction);
      return;
    }

    var flyX   = direction === 'approve' ?  '110vw' : '-110vw';
    var flyRot = direction === 'approve' ?  30       : -30;

    card.style.transition = 'transform 0.35s cubic-bezier(0.25,0.46,0.45,0.94), opacity 0.25s ease';
    card.style.transform  = 'translateX(' + flyX + ') rotate(' + flyRot + 'deg)';
    card.style.opacity    = '0';

    card.addEventListener('transitionend', function handler() {
      card.removeEventListener('transitionend', handler);
      submitAction(direction);
    });
  }

  // ---------------------------------------------------------------------------
  // submitAction — trigger the HTMX form
  // ---------------------------------------------------------------------------
  function submitAction(direction) {
    var deck = document.getElementById('job-card-deck');
    if (!deck) return;
    var form = deck.querySelector('form[data-action="' + direction + '"]');
    if (!form) return;
    if (window.htmx) {
      htmx.trigger(form, 'submit');
    } else {
      form.submit();
    }
  }

  // ---------------------------------------------------------------------------
  // snapBack — reset card to original position with spring easing
  // ---------------------------------------------------------------------------
  function snapBack(card) {
    card.style.transition = 'transform 0.40s cubic-bezier(0.34,1.56,0.64,1)';
    card.style.transform  = '';
    card.style.opacity    = '';

    var approveOverlay = card.querySelector('.job-card-overlay-approve');
    var rejectOverlay  = card.querySelector('.job-card-overlay-reject');
    if (approveOverlay) approveOverlay.style.opacity = '';
    if (rejectOverlay)  rejectOverlay.style.opacity  = '';

    card.addEventListener('transitionend', function handler() {
      card.removeEventListener('transitionend', handler);
      card.style.transition = '';
    });
  }

  // ---------------------------------------------------------------------------
  // expandSummary — toggle expanded state on the summary paragraph
  // ---------------------------------------------------------------------------
  function expandSummary(card) {
    var summary = card.querySelector('.job-card-summary');
    var btn     = card.querySelector('[data-expand-summary]');
    if (!summary) return;
    var expanded = summary.classList.toggle('expanded');
    if (btn) btn.textContent = expanded ? 'Show less' : 'Show more';
  }

  // ---------------------------------------------------------------------------
  // moveFocusAfterSwap — move keyboard focus to the next card or refresh button
  // ---------------------------------------------------------------------------
  function moveFocusAfterSwap() {
    var next = document.querySelector('.job-card-active .job-card-link');
    if (next) { next.focus(); return; }
    var refresh = document.querySelector('.job-card-empty button');
    if (refresh) refresh.focus();
  }

  // ---------------------------------------------------------------------------
  // Event listeners
  // ---------------------------------------------------------------------------
  document.addEventListener('htmx:afterSwap', function (e) {
    if (e.detail.target && e.detail.target.id === 'job-card-deck') {
      initDeck();
      moveFocusAfterSwap();
    }
  });

  document.addEventListener('DOMContentLoaded', initDeck);

})();
