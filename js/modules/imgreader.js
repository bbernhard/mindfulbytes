function randomInt(min, max) {
	return Math.floor(Math.random() * (max - min + 1) + min);
}


function getDataForDate(api, date) {
	api.getDataForDate("imgreader", date)
		.then(function(data) {
			return data;
		})
		.catch(function() {
			console.log("exception");
		});
}

function displayData(api, data, date) {
	var imageUrl = api.getImageUrlFromId("imgreader", data.response.uuid);
	$("#img").attr("src", imageUrl);

	$("#when").text(moment(date).fromNow() + " this picture was taken");
	$("#whenDetailed").text(date);
}

$(document).ready(function() {
    document.title = "MindfulBytes";
	
	
	currentDate = moment().format("MM-DD");
    api = new MindfulBytesApi("http://127.0.0.1:8085");
	
	
	api.getDataForDay("imgreader", currentDate)
        .then(function(data) {
			if(data.statusCode === 404) { //nothing found, pick a random image
				api.getDates("imgreader")
					.then(function(data) {
						if(data.length > 0) {
							var randomDate = data[randomInt(0, data.length -1)];
							console.log(randomDate)
							api.getDataForDate("imgreader", randomDate)
								.then(function(data) {
									displayData(api, data, randomDate);
								})
								.catch(function() {
									console.log("exception");
								});
						}
					})
					.catch(function() {
						console.log("error");
					});
			} else { //found something for today
				displayData(api, data, moment().format("YYYY-MM-DD"));
			}
        })
    	.catch(function() {
    		console.log("catch")
		});
});
