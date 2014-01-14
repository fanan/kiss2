function checkUrl(tabId, changeInfo, tab) {
  var result = tab.url.match(/tv.sohu.com\/\d+\/n\d+\.shtml/gi);
  if (result === null) {
    chrome.pageAction.hide(tabId);
  } else {
    chrome.pageAction.show(tabId);
  }
}

function addTask() {
  chrome.tabs.getSelected(null, function(tab){
    $.post("http://127.0.0.1:3000/api/sohu/add", {"url": tab.url}, function(data) {
      alert(data);
    });
  });
}

//chrome.browserAction.onClicked.addListener(say);
chrome.tabs.onUpdated.addListener(checkUrl);
chrome.pageAction.onClicked.addListener(addTask);
