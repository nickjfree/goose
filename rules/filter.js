var filters = ['baidu', 'shifen', 'csdn', 'qq', 'libp2p', 'z2pyw', 'ddys.pro'];
var filterRegions = ['CN'];

function isIPv4(str) {
  var ipv4Regex = /^(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$/;

  return ipv4Regex.test(str);
}

// Define the main function to match a domain
function matchDomain(domain) {
  if (isIPv4(domain)) {
    var country = getCountry(domain); 
    return filterRegions.indexOf(country) !== -1
  }
  else if (filters.some(function(name) {
    return domain.indexOf(name) !== -1;
  })) {
    return true;
  }
  return false;
}