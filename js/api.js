var MindfulBytesApi = (function() {
    function MindfulBytesApi(baseUrl, apiVersion = "v1") {
        this.baseUrl = baseUrl;
		this.apiVersion = apiVersion;
    };

	MindfulBytesApi.prototype.getPlugins = function(name = null) {
		var inst = this;
		return new Promise(function(resolve, reject) {
			var url = inst.baseUrl + "/" + inst.apiVersion + "/plugins";
			if(name !== null)
				url += "?name=" + name;
			var xhr = new XMLHttpRequest();
			xhr.responseType = "json";
			xhr.open("GET", url);
			xhr.onload = function() {
				var jsonResponse = xhr.response;
				resolve(jsonResponse);
			}
			xhr.onerror = reject;
			xhr.send();
		});
    }

	MindfulBytesApi.prototype.getImageUrlFromId = function(name, imageId, size="800x600") {
		return this.baseUrl + "/" + this.apiVersion + "/plugins/" + name + "/images/" + imageId + "?format=jpg&size=" + size;
	}

	MindfulBytesApi.prototype.getDataForFullDate = function(name, fullDate) {
		var inst = this;
		return new Promise(function(resolve, reject) {
			var url = inst.baseUrl + "/" + inst.apiVersion + "/topics/" + name + "/fulldates/" + fullDate;
			var xhr = new XMLHttpRequest();
			xhr.responseType = "json";
			xhr.open("GET", url);
			xhr.onload = function() {
				resolve({statusCode: xhr.status, response: xhr.response});
			}
			xhr.onerror = reject;
			xhr.send();
		});
    }

	MindfulBytesApi.prototype.getDataForDate = function(name, date) {
		var inst = this;
		return new Promise(function(resolve, reject) {
			var url = inst.baseUrl + "/" + inst.apiVersion + "/topics/" + name + "/dates/" + date;
			var xhr = new XMLHttpRequest();
			xhr.responseType = "json";
			xhr.open("GET", url);
			xhr.onload = function() {
				resolve({statusCode: xhr.status, response: xhr.response});
			}
			xhr.onerror = reject;
			xhr.send();
		});
    }

	MindfulBytesApi.prototype.getDates = function(name) {
		var inst = this;
		return new Promise(function(resolve, reject) {
			var url = inst.baseUrl + "/" + inst.apiVersion + "/topics/" + name + "/dates/";
			var xhr = new XMLHttpRequest();
			xhr.responseType = "json";
			xhr.open("GET", url);
			xhr.onload = function() {
				var jsonResponse = xhr.response;
				resolve(jsonResponse);
			}
			xhr.onerror = reject;
			xhr.send();
		});
    }

	MindfulBytesApi.prototype.getFullDates = function(name) {
		var inst = this;
		return new Promise(function(resolve, reject) {
			var url = inst.baseUrl + "/" + inst.apiVersion + "/topics/" + name + "/fulldates/";
			var xhr = new XMLHttpRequest();
			xhr.responseType = "json";
			xhr.open("GET", url);
			xhr.onload = function() {
				var jsonResponse = xhr.response;
				resolve(jsonResponse);
			}
			xhr.onerror = reject;
			xhr.send();
		});
    }
	
	return MindfulBytesApi;
}());
