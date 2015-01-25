
"use strict";

// Query

var queryElement = document.getElementById("query");
var submitElement = document.getElementById("submitQuery");

if (submitElement && queryElement) {
  submitElement.updateState = function() {
    var value = queryElement.value;
    submitElement.disabled = value.length < 3;
  };

  submitElement.updateState();

  queryElement.addEventListener("input", function(e) {
    submitElement.updateState();
  }, false);
}
