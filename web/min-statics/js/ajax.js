FCTDecminalLength=8;function getRequest(a,d){var b=new XMLHttpRequest;b.onreadystatechange=function(){4==b.readyState&&d(b.response)};b.open("GET","/GET?request="+a,!0);b.send()}function postRequest(a,d,b){var c=new XMLHttpRequest;c.onreadystatechange=function(){4==c.readyState&&b(c.response)};var e=new FormData;e.append("request",a);e.append("json",d);c.open("POST","/POST");c.send(e)}$(window).load(function(){updateBalances()});setInterval(updateBalances,5E3);
function updateBalances(){getRequest("balances",function(a){obj=JSON.parse(a);"none"==obj.Error&&($("#ec-balance").text(obj.Content.EC),fcBal=formatFC(obj.Content.FC),$("#factoid-balance").text(fcBal[0]+"."),1<fcBal.length?$("#factoid-balance-trailing").text(fcBal[1]):$("#factoid-balance-trailing").text(0))})}function formatFC(a){dec=FCTNormalize(a);decStr=dec.toString();return decSplit=decStr.split(".")}function FCTNormalize(a){return Number((a/1E8).toFixed(FCTDecminalLength))}
function SetGeneralError(a){$("#success-zone").slideUp(100);$("#error-zone").text(a);$("#error-zone").slideDown(100)}function SetGeneralSuccess(a){$("#error-zone").slideUp(100);$("#success-zone").text(a);$("#success-zone").slideDown(100)};
