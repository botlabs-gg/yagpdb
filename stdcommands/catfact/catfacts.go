package catfact

// Cat facts collected from vaious sources since the api i used shut down

var Catfacts = []string{
	`Americans really love their pet cats. There are approximately 14 million more house cats than dogs in the US.`,
	`Male cats are called toms and females are called queens or mollies.`,
	`The saying, 'A cat always lands on its feet' isn't just an old myth. Some cats have fallen more than 320 metres onto concrete and come away unharmed.`,
	`There's a reason why cats are likely to survive high falls – they have more time to prepare for the landing.`,
	`It's not a flock, it's not a herd – a group of cats together is known as a 'clowder'.`,
	`According to statistics, cat owners are healthier than those without cats. The risk of heart attack is cut by a third among people who have a pet cat.`,
	`Just as a dog's bark has several different meanings, a cat may purr because it is nervous, happy or feeling unwell.`,
	`It's common knowledge that cats are fans of milk, but many of them are lactose intolerant. This means that they are actually allergic to milk.`,
	`Garfield, the lasagne-loving feline, was featured in the Guinness Book of World Records for being the most widely published cartoon.`,
	`Cats are thought to be pretty smart, and with a brain 90% similar to the human brain, it's no surprise.`,
	`When Abraham Lincoln was US President, he had four pet cats which lived with him in the White House.`,
	`Felicette was the first feline to make a trip to space. She luckily survive the mission and was nicknamed 'Astrocat'.`,
	`If you think of yourself as a 'cat person', you're among 11.5% of people in the world.`,
	`If you're a male with a pet cat, you're more likely to find true love. This is due to the fact that people view cat owners are kind, trustworthy and sensitive.`,
	`The record for the largest number of kittens in the same litter was 19.`,
	`Over her lifetime, a cat called Dusty had a total of 420 kittens.`,
	`The oldest cat to give birth to kittens was called Kitty. She was 30 years old when she birthed her last kittens.`,
	`**Bagpuss** was a 1999 TV show which featured an old cloth cat. In 2001, it came fourth in a poll of the greatest kids' TV shows.`,
	`While people dote over their cats in the West, around 4 million cats are killed and eaten over in Asia.`,
	`There are about 70 different cat breeds and a staggering 500 million pet cats in the world.`,
	`Ancient Egyptians worshipped a goddess who was half cat and half woman.`,
	`In Ancient Egypt, civilians would suffer a severe punishment if they hurt a cat.`,
	`Ever noticed your cat sleeping most of the time? On average, cats sleep for about 16 hours per day.`,
	`Kittens sleep even more often, since growth hormones are released when they are napping.`,
	`The front paws of a cat are different from the back paws. They have five toes on the front but only four on the back`,
	`Some cats are known as 'polydactyl' and have extra toes. Some polydactyl have as many as eight toes per paw.`,
	`Unlike dogs, cats have no sense of what is sweet. No wonder they never seem happy with cakes!`,
	`Did you think the cat flap was an ultra-modern idea? Isaac Newton is credited with the invention of the cat flap – something which most cat owners now have.`,
	`Isaac Newton himself had a cat called Spithead, which influenced his invention. Spithead went on to have kittens, who all got their very own cat flap.`,
	`Many cat owners take their pets to the vet to be neutered, and by doing so, increase the life expectancy of their pets by 2-3 years.`,
	`Cats can see very well in the dark, which explains why they are always wandering around at night.`,
	`Adolf Hitler hated cats, so there's another reason why you shouldn't like him.`,
	`Taurine is an amino acid which can be found in cat food. Without this substance, your pet cat would eventually go blind.`,
	`House cats can run at a speed of 30 miles per hour.`,
	`The kidneys of a cat are quite amazing, since they can filter water before using it. This means that a cat can drink sea water and the salt will be filtered out.`,
	`Those cute furry bits inside a cat's ear are called 'ear furnishings'. They ensure that dirt doesn't go inside and also helps them to hear well.`,
	`Cats have a great sense of hearing and can hear ultrasonic noises. They could potentially hear dolphins`,
	`It's not just humans who are right-handed or left-handed. Most female cats prefer using their right paw, while male are more likely to be 'left-pawed'.`,
	`A 'haw' is the third eyelid of a cat, which can only be seen when the cat isn't well.`,
	`They might have an extra eyelid, but they don't have any eyelashes.`,
	`The original version of Cinderella in Italy featured a cat as the fairy godmother.`,
	`Dogs make 10 different sounds, while cats can make as many as 100 different noises.`,
	`Cats seem like such sweet creatures, but approximately 40,000 people suffer from cat bites every single year in America.`,
	`When you see a cat rubbing up against a human, it is being affectionate but also marking its territory to make other cats aware.`,
	`The cat family consists of many different animals, but the largest is the Siberian Tiger, which can be as large as 12 feet long.`,
	`The black-footed cat is the smallest wild cat, at just 20 inches long.`,
	`The wealthiest cat ever was Blackie, a multi-millionaire. It owned £15 million after its rich owner died.`,
	`London was the home of the first ever cat show. It took place in 1871 and started a trend which has continued ever since.`,
	`While cats like to catch mice and eat them, it's not necessarily a good meal. They can contract tapeworm through eating these rodents.`,
	`Cats' hearts beat at a rate of 110-140 beats per minutes – around twice as fast as the average human.`,
	`Ancient Egyptian cat owners would shave off their eyebrows when mourning for their dead kitties.`,
	`'Mau' is the Egyptian word for cat, and the oldest surviving cat breed is known as the Egyptian Mau, translated to mean the 'Egyptian cat'.`,
	`While people are hesitant to buy cats of opposite sexes, they will actually get along better than those of the same sex.`,
	`If you own a cat, you should feed them 10-20 small meals every day, rather than fewer and larger meals.`,
	`Over the Christmas season, cats should avoid poinsettias as they are poisonous.`,
	`Stray and feral cats which live outdoors have a life span of around 4 years. Those which live indoors can live up to 16 years or more.`,
	`Cats are lovers of fish, but too much tuna can cause them to become addicted to this meat.`,
	`They use their tongues to thoroughly clean themselves, but they also use them to get rid of human scent.`,
	`When cats are first born, they have blue eyes. Over time, the colour changes.`,
	`Kittens also have much sharper teeth than adult cats. Their teeth become blunt when they are around 6 months old.`,
	`Cats which have blue eyes for the duration of their lives are likely to be deaf.`,
	`They usually hate water, but the Turkish Van cats actually enjoy getting wet!`,
	`Unlike many other animals, cats cannot produce fat on their own. It's important to give your pet a balanced diet which includes good fats.`,
	`Just as a human's fingerprints are unique, each cat has a completely different nose.`,
	`Many people are allergic to cats, but cats can actually be allergic to humans too. Around one in 200 domestic cats suffer from asthma due to smoke, dust and other particles within houses.`,
	`As long as you introduce your cat to your dog before they are both six months old, they should get along well.`,
	`Although studies suggested that cats didn't enjoy being stroked by humans, further research has proven that they do in fact like it.`,
	`A cat's brain is so quick that even a super-computer in 2015 couldn't beat it.`,
	`Feral cats will go out exploring more often and much further than house cats. House cats can normally be found within the area they live.`,
	`It is thought that 55% of house cats are obese due to overeating.`,
	`Alzheimer's disease can be found in cats, just as it can be found in humans.`,
	`Cats were first kept as pets about 5,000 years ago in China. Farmers were the first to realise that felines could be kept in the home.`,
	`In 2011, scientists concluded that your pet cat may become very ill if you interfere with its routine.`,
	`The high-pitched cry which you may have heard from your cat is an attempt for cats to get their own way. The cry is similar to that of a new-born baby.`,
	`Cats enjoy sitting on warm objects, which is probably why your kitty likes to sit on your computer so often.`,
	`It's a well-known fact that cats are very fussy creatures, and if your pet doesn't always drink water from its water bowl, it may not like the shape of the bowl!`,
	`Cats might look harmless, but they have worked together to make over 30 different species extinct. Even house cats love hunting, and have contributed heavily to this figure.`,
	`They only sweat through their paws, as this is the only place which has sweat glands.`,
	`In the 1950's, Disneyland bought several cats in order to hunt mice at night. There are now more than 200 felines at the amusement park.`,
	`Their whiskers are used to measure gaps and openings. They allow the cat to work out whether or not they will fit through spaces.`,
	`Historians believe that every species of the cat family came from one of just five different wild cats from Africa.`,
	`Female cats can breed with several males when they are in heat. These means that a litter of kittens could potentially have a few different fathers!`,
	`A cat can rotate each of its ears separately. Each ear has a total of 32 muscles.`,
	`Don't feed chocolate to your cat – it's poisonous.`,
	`Cats are pretty amazing at jumping; they can jump up to seven times higher than the length of their tail.`,
	`There's a charming reason your cat brings dead mice to you. It means that your pet likes you!`,
	`They cannot survive eating a vegetarian diet, which is why it's important to feed your cat meat.`,
	`Cats can find their way home even if they have travelled miles away.`,
	`Black cats are the least likely to be adopted from an animal home, although people love buying jet black kittens.`,
	`A cat's tail will quiver if it is near somebody it loves.`,
	`Think your cat purrs a lot? Cats purr up to 26 times per second.`,
	`Felines enjoy spending time alone. Unlike dogs, they don't need much attention and can be very happy without any companionship.`,
	`You shouldn't feed many cats from the same food bowl. It is very likely that some of them will refuse to eat from it, choosing rather to eat alone.`,
	`Don't stare at your cat for long periods of time. This is seen as threatening and will make it feel uneasy.`,
	`There are many plants which are poisonous for cats, although parsley, sage and other herbs are among a cat's favourite foods.`,
	`If you put a collar on your cat, make sure it's not too tight. You should be able to fit two fingers between the collar and your cat's neck, or you could risk strangling it.`,
	`The fear of cats is known as Ailurophobia.`,
	`A town called Talkeetna in Alaska had a cat as mayor for 15 years.`,
	`The record for the world's longest cat was 48.5 inches.`,
	`Cats do recognise their owners' voices, but often act like they don't care.`,
	`Unlike dogs, cats do not have a sweet tooth. Scientists believe this is due to a mutation in a key taste receptor.`,
	`When a cat chases its prey, it keeps its head level. Dogs and humans bob their heads up and down.`,
	`The technical term for a cat’s hairball is a “bezoar.”`,
	`A group of cats is called a “clowder.”`,
	`A cat can’t climb head first down a tree because every claw on a cat’s paw points the same way. To get down from a tree, a cat must back down.`,
	`Cats make about 100 different sounds. Dogs make only about 10.`,
	`Many people in China consider cats a "warming" food that is perfect to eat during the winter`,
	`Every year, nearly four million cats are eaten in Asia.`,
	`There are more than 500 million domestic cats in the world, with approximately 40 recognized breeds.`,
	`Approximately 24 cat skins can make a coat.`,
	`While it is commonly thought that the ancient Egyptians were the first to domesticate cats, the oldest known pet cat was recently found in a 9,500-year-old grave on the Mediterranean island of Cyprus. This grave predates early Egyptian art depicting cats by 4,000 years or more.`,
	`CW // animal cruelty/death ||During the time of the Spanish Inquisition, Pope Innocent VIII condemned cats as evil and thousands of cats were burned. Unfortunately, the widespread killing of cats led to an explosion of the rat population, which exacerbated the effects of the Black Death.||`,
	`CW // animal cruelty/death ||During the Middle Ages, cats were associated with withcraft, and on St. John’s Day, people all over Europe would stuff them into sacks and toss the cats into bonfires. On holy days, people celebrated by tossing cats from church towers.||`,
	`The first cat in space was a French cat named Felicette (a.k.a. “Astrocat”) In 1963, France blasted the cat into outer space. Electrodes implanted in her brains sent neurological signals back to Earth. She survived the trip.`,
	`The group of words associated with cat (catt, cath, chat, katze) stem from the Latin catus, meaning domestic cat, as opposed to feles, or wild cat.`,
	`The term “puss” is the root of the principal word for “cat” in the Romanian term pisica and the root of secondary words in Lithuanian (puz) and Low German puus. Some scholars suggest that “puss” could be imitative of the hissing sound used to get a cat’s attention. As a slang word for the female pudenda, it could be associated with the connotation of a cat being soft, warm, and fuzzy.`,
	`Approximately 40,000 people are bitten by cats in the U.S. annually.`,
	`Cats are the world's most popular pets, outnumbering dogs by as many as three to one`,
	`Cats are North America’s most popular pets: there are 73 million cats compared to 63 million dogs. Over 30% of households in North America own a cat.`,
	`According to Hebrew legend, Noah prayed to God for help protecting all the food he stored on the ark from being eaten by rats. In reply, God made the lion sneeze, and out popped a cat.`,
	`A cat’s hearing is better than a dog’s. And a cat can hear high-frequency sounds up to two octaves higher than a human.`,
	`A cat can travel at a top speed of approximately 31 mph (49 km) over a short distance.`,
	`A cat rubs against people not only to be affectionate but also to mark out its territory with scent glands around its face. The tail area and paws also carry the cat’s scent.`,
	`Researchers are unsure exactly how a cat purrs. Most veterinarians believe that a cat purrs by vibrating vocal folds deep in the throat. To do this, a muscle in the larynx opens and closes the air passage about 25 times per second.`,
	`When a family cat died in ancient Egypt, family members would mourn by shaving off their eyebrows. They also held elaborate funerals during which they drank wine and beat their breasts. The cat was embalmed with a sculpted wooden mask and the tiny mummy was placed in the family tomb or in a pet cemetery with tiny mummies of mice.`,
	`In 1888, more than 300,000 mummified cats were found an Egyptian cemetery. They were stripped of their wrappings and carted off to be used by farmers in England and the U.S. for fertilizer.`,
	`Most cats give birth to a litter of between one and nine kittens. The largest known litter ever produced was 19 kittens, of which 15 survived.`,
	`Smuggling a cat out of ancient Egypt was punishable by death. Phoenician traders eventually succeeded in smuggling felines, which they sold to rich people in Athens and other important cities.`,
	`The earliest ancestor of the modern cat lived about 30 million years ago. Scientists called it the Proailurus, which means “first cat” in Greek. The group of animals that pet cats belong to emerged around 12 million years ago.`,
	`The biggest wildcat today is the Siberian Tiger. It can be more than 12 feet (3.6 m) long (about the size of a small car) and weigh up to 700 pounds (317 kg).`,
	`Cats have 300 million neurons; dogs have about 160 million`,
	`A cat’s brain is biologically more similar to a human brain than it is to a dog’s. Both humans and cats have identical regions in their brains that are responsible for emotions.`,
	`Many Egyptians worshipped the goddess Bast, who had a woman’s body and a cat’s head.`,
	`Mohammed loved cats and reportedly his favorite cat, Muezza, was a tabby. Legend says that tabby cats have an “M” for Mohammed on top of their heads because Mohammad would often rest his hand on the cat’s head.`,
	`While many parts of Europe and North America consider the black cat a sign of bad luck, in Britain and Australia, black cats are considered lucky.`,
	`The most popular pedigreed cat is the Persian cat, followed by the Main Coon cat and the Siamese cat.`,
	`The smallest pedigreed cat is a Singapura, which can weigh just 4 lbs (1.8 kg), or about five large cans of cat food. The largest pedigreed cats are Maine Coon cats, which can weigh 25 lbs (11.3 kg), or nearly twice as much as an average cat weighs.`,
	`Some cats have survived falls of over 65 feet (20 meters), due largely to their “righting reflex.” The eyes and balance organs in the inner ear tell it where it is in space so the cat can land on its feet. Even cats without a tail have this ability.`,
	`Some Siamese cats appear cross-eyed because the nerves from the left side of the brain go to mostly the right eye and the nerves from the right side of the brain go mostly to the left eye. This causes some double vision, which the cat tries to correct by “crossing” its eyes.`,
	`Researchers believe the word “tabby” comes from Attabiyah, a neighborhood in Baghdad, Iraq. Tabbies got their name because their striped coats resembled the famous wavy patterns in the silk produced in this city.`,
	`Cats have "nine lives" thanks to a flexible spine and powerful leg and back muscles`,
	`A cat can jump up to five times its own height in a single bound.`,
	`Cats hate the water because their fur does not insulate well when it’s wet. The Turkish Van, however, is one cat that likes swimming. Bred in central Asia, its coat has a unique texture that makes it water resistant.`,
	`The Egyptian Mau is probably the oldest breed of cat. In fact, the breed is so ancient that its name is the Egyptian word for “cat.”`,
	`The first commercially cloned pet was a cat named "Little Nicky." He cost his owner $50,000, making him one of the most expensive cats ever.`,
	`A cat usually has about 12 whiskers on each side of its face.`,
	`A cat’s eyesight is both better and worse than humans. It is better because cats can see in much dimmer light and they have a wider peripheral view. It’s worse because they don’t see color as well as humans do. Scientists believe grass appears red to cats.`,
	`Spanish-Jewish folklore recounts that Adam’s first wife, Lilith, became a black vampire cat, sucking the blood from sleeping babies. This may be the root of the superstition that a cat will smother a sleeping baby or suck out the child’s breath.`,
	`Perhaps the most famous comic cat is the Cheshire Cat in Lewis Carroll’s Alice in Wonderland. With the ability to disappear, this mysterious character embodies the magic and sorcery historically associated with cats.`,
	`The smallest wildcat today is the Black-footed cat. The females are less than 20 inches (50 cm) long and can weigh as little as 2.5 lbs (1.2 kg).`,
	`On average, cats spend 2/3 of every day sleeping. That means a nine-year-old cat has been awake for only three years of its life.`,
	`Most cats sleep around 16 hours a day`,
	`In the original Italian version of Cinderella, the benevolent fairy godmother figure was a cat.`,
	`The little tufts of hair in a cat’s ear that help keep out dirt direct sounds into the ear, and insulate the ears are called “ear furnishings.”`,
	`The ability of a cat to find its way home is called “psi-traveling.” Experts think cats either use the angle of the sunlight to find their way or that cats have magnetized cells in their brains that act as compasses.`,
	`Isaac Newton invented the cat flap. Newton was experimenting in a pitch-black room. Spithead, one of his cats, kept opening the door and wrecking his experiment. The cat flap kept both Newton and Spithead happy.`,
	`The world’s rarest coffee, Kopi Luwak, comes from Indonesia where a wildcat known as the luwak lives. The cat eats coffee berries and the coffee beans inside pass through the stomach. The beans are harvested from the cat’s dung heaps and then cleaned and roasted. Kopi Luwak sells for about $500 for a 450 g (1 lb) bag.`,
	`A cat’s jaw can’t move sideways, so a cat can’t chew large chunks of food.`,
	`A cat almost never meows at another cat, mostly just humans. Cats typically will spit, purr, and hiss at other cats.`,
	`Like humans, cats tend to favor one paw over another`,
	`Female cats tend to be right pawed, while male cats are more often left pawed. Interestingly, while 90% of humans are right handed, the remaining 10% of lefties also tend to be male.`,
	`A cat’s back is extremely flexible because it has up to 53 loosely fitting vertebrae. Humans only have 34.`,
	`All cats have claws, and all except the cheetah sheath them when at rest.`,
	`Two members of the cat family are distinct from all others: the clouded leopard and the cheetah. The clouded leopard does not roar like other big cats, nor does it groom or rest like small cats. The cheetah is unique because it is a running cat; all others are leaping cats. They are leaping cats because they slowly stalk their prey and then leap on it.`,
	`A cat lover is called an Ailurophilia (Greek: cat+lover).`,
	`In Japan, cats are thought to have the power to turn into super spirits when they die. This may be because according to the Buddhist religion, the body of the cat is the temporary resting place of very spiritual people.i`,
	`Most cats had short hair until about 100 years ago, when it became fashionable to own cats and experiment with breeding.`,
	`One reason that kittens sleep so much is because a growth hormone is released only during sleep.`,
	`Cats have about 130,000 hairs per square inch (20,155 hairs per square centimeter).`,
	`The heaviest cat on record is Himmy, a Tabby from Queensland, Australia. He weighed nearly 47 pounds (21 kg). He died at the age of 10.`,
	`The oldest cat on record was Crème Puff from Austin, Texas, who lived from 1967 to August 6, 2005, three days after her 38th birthday. A cat typically can live up to 20 years, which is equivalent to about 96 human years.`,
	`The lightest cat on record is a blue point Himalayan called Tinker Toy, who weighed 1 pound, 6 ounces (616 g). Tinker Toy was 2.75 inches (7 cm) tall and 7.5 inches (19 cm) long.`,
	`Approximately 1/3 of cat owners think their pets are able to read their minds.`,
	`The tiniest cat on record is Mr. Pebbles, a 2-year-old cat that weighed 3 lbs (1.3 k) and was 6.1 inches (15.5 cm) high.`,
	`A commemorative tower was built in Scotland for a cat named Towser, who caught nearly 30,000 mice in her lifetime.`,
	`In the 1750s, Europeans introduced cats into the Americas to control pests.`,
	`The first cat show was organized in 1871 in London. Cat shows later became a worldwide craze.`,
	`The first cartoon cat was Felix the Cat in 1919. In 1940, Tom and Jerry starred in the first theatrical cartoon “Puss Gets the Boot.” In 1981 Andrew Lloyd Weber created the musical Cats, based on T.S. Eliot’s Old Possum’s Book of Practical Cats.`,
	`The normal body temperature of a cat is between 100.5 ° and 102.5 °F. A cat is sick if its temperature goes below 100 ° or above 103 °F.`,
	`A cat has 230 bones in its body. A human has 206. A cat has no collarbone, so it can fit through any opening the size of its head.`,
	`Cats control the outer ear using 32 muscles; humans use 6`,
	`Cats have 32 muscles that control the outer ear (humans have only 6). A cat can independently rotate its ears 180 degrees.`,
	`A cat’s nose pad is ridged with a unique pattern, just like the fingerprint of a human.`,
	`If they have ample water, cats can tolerate temperatures up to 133 °F.`,
	`Foods that should not be given to cats include onions, garlic, green tomatoes, raw potatoes, chocolate, grapes, and raisins. Though milk is not toxic, it can cause an upset stomach and gas. Tylenol and aspirin are extremely toxic to cats, as are many common houseplants. Feeding cats dog food or canned tuna that’s for human consumption can cause malnutrition.`,
	`A 2007 Gallup poll revealed that both men and women were equally likely to own a cat.`,
	`A cat’s heart beats nearly twice as fast as a human heart, at 110 to 140 beats a minute.`,
	`In just seven years, a single pair of cats and their offspring could produce a staggering total of 420,000 kittens.`,
	`Relative to its body size, the clouded leopard has the biggest canines of all animals’ canines. Its dagger-like teeth can be as long as 1.8 inches (4.5 cm).`,
	`Cats spend nearly 1/3 of their waking hours cleaning themselves.`,
	`Grown cats have 30 teeth. Kittens have about 26 temporary teeth, which they lose when they are about 6 months old.`,
	`Cat paws act as tempetature regulators, shock absorbers, hunting and grooming tools, sensors, and more`,
	`Cats don’t have sweat glands over their bodies like humans do. Instead, they sweat only through their paws.`,
	`A cat called Dusty has the known record for the most kittens. She had more than 420 kittens in her lifetime.`,
	`The largest cat breed is the Ragdoll. Male Ragdolls weigh between 12 and 20 lbs (5.4-9.0 k). Females weigh between 10 and 15 lbs (4.5-6.8 k).`,
	`Cats are extremely sensitive to vibrations. Cats are said to detect earthquake tremors 10 or 15 minutes before humans can.`,
	`In contrast to dogs, cats have not undergone major changes during their domestication process.`,
	`A female cat is called a queen or a molly.`,
	`In the 1930s, two Russian biologists discovered that color change in Siamese kittens depend on their body temperature. Siamese cats carry albino genes that work only when the body temperature is above 98° F. If these kittens are left in a very warm room, their points won’t darken and they will stay a creamy white.`,
	`There are up to 60 million feral cats in the United States alone.`,
	`The oldest cat to give birth was Kitty who, at the age of 30, gave birth to two kittens. During her life, she gave birth to 218 kittens.`,
	`The most traveled cat is Hamlet, who escaped from his carrier while on a flight. He hid for seven weeks behind a panel on the airplane. By the time he was discovered, he had traveled nearly 373,000 miles (600,000 km).`,
	`In Holland’s embassy in Moscow, Russia, the staff noticed that the two Siamese cats kept meowing and clawing at the walls of the building. Their owners finally investigated, thinking they would find mice. Instead, they discovered microphones hidden by Russian spies. The cats heard the microphones when they turned on.`,
	`The most expensive cat was an Asian Leopard cat (ALC)-Domestic Shorthair (DSH) hybrid named Zeus. Zeus, who is 90% ALC and 10% DSH, has an asking price of £100,000 ($154,000).`,
	`The cat who holds the record for the longest non-fatal fall is Andy. He fell from the 16th floor of an apartment building (about 200 ft/.06 km) and survived.`,
	`The richest cat is Blackie who was left £15 million by his owner, Ben Rea.`,
	`The claws on the cat’s back paws aren’t as sharp as the claws on the front paws because the claws in the back don’t retract and, consequently, become worn.`,
	`Cats are the most popular pet in the United States: There are 88 million pet cats and 74 million dogs.`,
	`There are cats who have survived falls from over 32 stories (320 meters) onto concrete.`,
	`Cats have over 20 muscles that control their ears.`,
	`Cats sleep 70% of their lives.`,
	`A cat has been mayor of Talkeetna, Alaska, for 15 years. His name is Stubbs.`,
	`A cat ran for mayor of Mexico City in 2013.`,
	`In tigers and tabbies, the middle of the tongue is covered in backward-pointing spines, used for breaking off and gripping meat.`,
	`When cats grimace, they are usually "taste-scenting." They have an extra organ that, with some breathing control, allows the cats to taste-sense the air.`,
	`The world's largest cat measured 48.5 inches long.`,
	`Owning a cat can reduce the risk of stroke and heart attack by a third.`,
	`The world's richest cat is worth $13 million after his human passed away and left her fortune to him.`,
	`Similarly, the frequency of a domestic cat's purr is the same at which muscles and bones repair themselves.`,
	`Adult cats only meow to communicate with humans,`,
	`Cats are often lactose intolerant, so stop givin' them milk!`,
	`The oldest cat video on YouTube dates back to 1894 (when it was made, not when it was uploaded, duh).`,
	`In the 1960s, the CIA tried to turn a cat into a bonafide spy by implanting a microphone into her ear and a radio transmitter at the base of her skull. She somehow survived the surgery but got hit by a taxi on her first mission.`,
	`Female cats are typically right-pawed while male cats are typically left-pawed.`,
	`Basically, cats have a lower social IQ than dogs but can solve more difficult cognitive problems when they feel like it.`,
	`Cats are magical. Here's a cute cat for you. :3 - https://giphy.com/gifs/JIX9t2j0ZTN9S`,
	`Cats have 1,000 times more data storage than an iPad.`,
	`Isaac Newton is credited with inventing the cat door.`,
	`A house cat is faster than Usain Bolt.`,
	`When cats leave their poop uncovered, it is a sign of aggression to let you know they don't fear you.`,
	`Cats use their whiskers to detect if they can fit through a space.`,
	`A cat's nose is ridged with a unique pattern, just like a human fingerprint.`,
	`Cats lick themselves to get *your* scent off, not theirs... Nice try though...`,
	`Ancient Egyptians shaved off their eyebrows to mourn the death of their cats.`,
	`Cat gut, used in tennis rackets and strings for musical instruments does not come from cats, but from sheep.`,
	`Cat urine glows under a black light.`,
	`A cat rubs against people to mark them as their territory.`,
	`Unlike humans, cats have kidneys that can filter out salt and use the water content to hydrate their bodies.`,
	`The furry tufts on the inside of cats’ ears are called "ear furnishings".`,
	`Cats can hear the ultrasonic noises that rodents (and dolphins) make to communicate.`,
	`The ridged pattern on a cat’s nose is as unique as a human fingerprint.`,
	`Cats can’t see directly below their noses. That’s why they miss food that’s right in front of them.`,
	`Isaac Newton invented the cat flap after his own cat, Spithead, kept opening the door and spoiling his light experiments.`,
	`Kittens start to dream when they’re about a week old.`,
	`Kittens sleep a lot because their bodies release a growth hormone only when they’re asleep.`,
	`Adult cats don’t release any particular key hormones during sleep. They just snooze all day because they *can*.`,
	`Cats sleep so much that by the time a cat is 9 years old, it will only have been awake for three years of its life.`,
	`Most female cats are right-pawed, and most male cats favour their left paws.`,
	`Cats sweat through the pads of their paws.`,
	`Cats have a third eyelid called a "haw". It’s generally only visible when they’re unwell.`,
	`Cats can run 3 mph faster than Usain Bolt.`,
}
